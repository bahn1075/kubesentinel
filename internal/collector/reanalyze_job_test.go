package collector

import (
	"errors"
	"testing"
)

// 중복 실행 방지: 실행 중인 작업은 다시 start되지 않아야 한다.
func TestReanalyzeJobsNoDuplicateRun(t *testing.T) {
	r := newReanalyzeJobs()

	if _, ok := r.start("inc-1"); !ok {
		t.Fatal("첫 start는 성공해야 한다")
	}
	if _, ok := r.start("inc-1"); ok {
		t.Error("실행 중인데 중복 start가 허용됐다")
	}
	// 다른 인시던트는 영향 없음
	if _, ok := r.start("inc-2"); !ok {
		t.Error("다른 인시던트는 시작 가능해야 한다")
	}

	// 끝난 뒤에는 재시도 가능
	r.finish("inc-1", nil)
	if _, ok := r.start("inc-1"); !ok {
		t.Error("완료된 작업은 다시 시작 가능해야 한다")
	}
}

func TestReanalyzeJobsStateTransitions(t *testing.T) {
	r := newReanalyzeJobs()

	// 기록 없음 → idle로 취급(핸들러가 판단)
	if _, ok := r.get("none"); ok {
		t.Error("없는 작업이 조회됐다")
	}

	r.start("ok")
	if j, _ := r.get("ok"); j.State != ReanalyzeRunning {
		t.Errorf("start 후 state=%s, want running", j.State)
	}
	r.finish("ok", nil)
	if j, _ := r.get("ok"); j.State != ReanalyzeDone {
		t.Errorf("finish(nil) 후 state=%s, want done", j.State)
	}

	r.start("bad")
	r.finish("bad", errors.New("LLM 연결 실패"))
	j, _ := r.get("bad")
	if j.State != ReanalyzeFailed {
		t.Errorf("finish(err) 후 state=%s, want failed", j.State)
	}
	if j.Error != "LLM 연결 실패" {
		t.Errorf("error=%q, 원인이 보존되지 않았다", j.Error)
	}
	if j.EndedAt.IsZero() {
		t.Error("EndedAt이 설정되지 않았다")
	}
}

// get이 복사본을 반환해 호출자가 내부 상태를 바꿀 수 없어야 한다.
func TestReanalyzeJobsGetReturnsCopy(t *testing.T) {
	r := newReanalyzeJobs()
	r.start("inc-1")

	j, _ := r.get("inc-1")
	j.State = "tampered"

	if again, _ := r.get("inc-1"); again.State != ReanalyzeRunning {
		t.Errorf("내부 상태가 외부에서 변경됐다: %s", again.State)
	}
}

// 동시 start 요청이 몰려도 하나만 통과해야 한다(중복 LLM 호출 방지).
func TestReanalyzeJobsConcurrentStart(t *testing.T) {
	r := newReanalyzeJobs()
	const n = 50
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() { _, ok := r.start("same"); results <- ok }()
	}
	won := 0
	for i := 0; i < n; i++ {
		if <-results {
			won++
		}
	}
	if won != 1 {
		t.Errorf("동시 start 중 %d개가 통과했다, want 1", won)
	}
}
