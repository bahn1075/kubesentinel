package collector

import (
	"fmt"
	"sync"
	"time"

	"kubesentinel-ai/internal/models"
)

// ReanalyzeState는 재분석 작업의 진행 상태입니다.
const (
	ReanalyzeRunning = "running"
	ReanalyzeDone    = "done"
	ReanalyzeFailed  = "failed"
)

// reanalyzeJob은 한 인시던트에 대한 재분석 작업의 상태입니다.
//
// 로컬 LLM은 분석에 수 분이 걸려 동기 응답이 프록시(nginx 기본 60초) 타임아웃에 걸린다.
// 그래서 요청은 202로 즉시 반환하고, 진행 상태를 여기에 담아 폴링으로 노출한다.
type reanalyzeJob struct {
	State     string    `json:"state"` // running | done | failed
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// ElapsedSec은 경과(또는 소요) 초입니다. 화면에 "분석 중 (12초)"처럼 표시한다.
func (j reanalyzeJob) ElapsedSec() int {
	end := j.EndedAt
	if end.IsZero() {
		end = time.Now()
	}
	return int(end.Sub(j.StartedAt).Seconds())
}

// reanalyzeJobs는 인시던트 ID → 작업 상태 맵입니다.
//
// 의도적으로 in-memory다. 프로세스가 재시작되면 맵이 비고, 화면은 "실행 중이 아님"으로
// 보이므로 다시 시도할 수 있다(DB에 running 플래그를 남기면 재시작 시 영구히 멈춘 것처럼 보인다).
// 단 replica가 여러 개면 상태가 공유되지 않는다(차트 기본 replicaCount=1).
type reanalyzeJobs struct {
	mu   sync.Mutex
	jobs map[string]*reanalyzeJob
}

func newReanalyzeJobs() *reanalyzeJobs {
	return &reanalyzeJobs{jobs: map[string]*reanalyzeJob{}}
}

// start는 작업을 running으로 등록한다. 이미 실행 중이면 (nil, false)를 반환한다(중복 실행 방지).
func (r *reanalyzeJobs) start(id string) (*reanalyzeJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if j, ok := r.jobs[id]; ok && j.State == ReanalyzeRunning {
		return nil, false
	}
	j := &reanalyzeJob{State: ReanalyzeRunning, StartedAt: time.Now()}
	r.jobs[id] = j
	return j, true
}

func (r *reanalyzeJobs) finish(id string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return
	}
	j.EndedAt = time.Now()
	if err != nil {
		j.State, j.Error = ReanalyzeFailed, err.Error()
		return
	}
	j.State = ReanalyzeDone
}

// get은 작업 상태의 복사본을 반환한다. 없으면 (zero, false).
func (r *reanalyzeJobs) get(id string) (reanalyzeJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return reanalyzeJob{}, false
	}
	return *j, true
}

// runReanalyze는 저장된 근거로 AI 진단을 다시 수행하고 결과를 영속화한다(goroutine에서 실행).
// 근거는 재수집하지 않는다 — LLM 연결이 그때 끊겨 있었을 뿐 수집된 근거는 유효하다.
func (s *WebhookServer) runReanalyze(id string, view models.IncidentView) {
	bundle := models.EvidenceBundleFromView(view)

	result, err := s.analyzeGated(bundle)
	if err != nil {
		fmt.Printf("[KubeSentinel] ⚠️  재분석 실패 (%s): %v\n", id, err)
		s.reanalyzeMgr().finish(id, fmt.Errorf("AI 연결을 확인하세요: %w", err))
		return
	}

	updated := models.NewIncidentView(bundle, result, "DiagnosisCompleted")
	updated.CreatedAt = view.CreatedAt // 최초 발생 시각 보존 — 목록 정렬(created_at)이 바뀌지 않게
	updated.PRURL = view.PRURL
	if err := s.Store.SaveIncident(updated); err != nil {
		fmt.Printf("[KubeSentinel] ⚠️  재분석 결과 저장 실패 (%s): %v\n", id, err)
		s.reanalyzeMgr().finish(id, fmt.Errorf("결과 저장 실패: %w", err))
		return
	}

	fmt.Printf("[KubeSentinel] ✅ 재분석 완료 (%s): %s\n", id, result.RootCause)
	s.reanalyzeMgr().finish(id, nil)
}
