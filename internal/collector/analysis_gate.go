package collector

import (
	"fmt"
	"time"

	"kubesentinel-ai/internal/diagnosis"
	"kubesentinel-ai/internal/models"
)

// analysisGate는 동시에 수행되는 AI 분석 개수를 제한한다.
//
// 로컬 모델(LM Studio·Ollama)은 동시 요청을 처리하지 못한다. alert가 몰리면
// (Alertmanager 폴러가 30초마다 여러 건을 띄운다) 개별 LLM 호출이 HTTP 타임아웃을
// 넘겨 전부 실패한다 — 실측: 4건 동시 실행 시 모두 "context deadline exceeded".
//
// 초과 요청은 거부하지 않고 순서를 기다린다. 분석은 이미 비동기라(webhook·폴러는
// goroutine, 재분석은 202 후 폴링) 대기가 HTTP 응답을 막지 않는다.
type analysisGate struct {
	slots chan struct{}
}

func newAnalysisGate(maxConcurrent int) *analysisGate {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	return &analysisGate{slots: make(chan struct{}, maxConcurrent)}
}

// analyze는 슬롯을 얻은 뒤 진단을 수행한다. 슬롯이 없으면 비워질 때까지 기다린다.
func (g *analysisGate) analyze(engine *diagnosis.Engine, b *models.EvidenceBundle) (*models.DiagnosisResult, error) {
	waited := time.Now()
	g.slots <- struct{}{}
	defer func() { <-g.slots }()

	if q := time.Since(waited); q > 2*time.Second {
		fmt.Printf("[KubeSentinel] ⏳ 분석 대기 %.0fs (동시 실행 제한, ai.max_concurrent): %s\n",
			q.Seconds(), b.IncidentID)
	}
	return engine.Analyze(b)
}

// gate는 분석 게이트를 반환한다(최초 사용 시 AI.MaxConcurrent로 초기화).
func (s *WebhookServer) gate() *analysisGate {
	s.gateOnce.Do(func() { s.gateV = newAnalysisGate(s.AI.MaxConcurrent) })
	return s.gateV
}

// analyzeGated는 동시 실행 제한을 적용해 진단을 수행한다.
// Engine.Analyze를 직접 부르지 말고 이 함수를 쓴다(webhook·폴러·재분석 공용).
func (s *WebhookServer) analyzeGated(b *models.EvidenceBundle) (*models.DiagnosisResult, error) {
	return s.gate().analyze(s.Engine, b)
}
