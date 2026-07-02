package collector

import (
	"fmt"

	"kubesentinel-ai/internal/config"
	"kubesentinel-ai/internal/models"
	"kubesentinel-ai/internal/runbook"
)

// Enricher는 EvidenceBundle에 Prometheus metric / Loki 로그를 보강합니다. (architecture.md §4.1)
// 수집은 best-effort다: 개별 소스 실패가 전체 RCA 흐름을 막지 않는다.
type Enricher struct {
	prom     *PrometheusClient
	loki     *LokiClient
	kube     *KubeCollector
	runbooks *runbook.Store
	logLines int
}

// NewEnricher는 설정 기반으로 Enricher를 구성합니다. 엔드포인트가 비어 있으면 해당 소스는 비활성화됩니다.
func NewEnricher(cfg config.CollectorConfig) *Enricher {
	return &Enricher{
		prom:     NewPrometheusClient(cfg.PrometheusURL),
		loki:     NewLokiClient(cfg.LokiURL),
		kube:     NewKubeCollector(), // in-cluster 아니면 nil (자동 skip)
		runbooks: runbook.Load(cfg.RunbookDir),
		logLines: cfg.LogLines,
	}
}

// Enrich는 bundle을 in-place로 보강합니다. 반환된 error는 부분 실패의 경고 목적이며,
// 호출자는 bundle을 그대로 진단에 사용해도 된다.
func (e *Enricher) Enrich(b *models.EvidenceBundle) {
	if b == nil {
		return
	}

	// 1. Prometheus: 재시작 횟수 + 메모리 사용량 (kube-state-metrics / cAdvisor 메트릭 기준, best-effort)
	if e.prom != nil && b.Namespace != "" {
		podMatch := b.Pod
		if podMatch == "" {
			podMatch = b.Workload
		}
		queries := map[string]string{
			"restarts":                 fmt.Sprintf(`kube_pod_container_status_restarts_total{namespace=%q,pod=~%q}`, b.Namespace, podMatch+".*"),
			"memory_working_set_bytes": fmt.Sprintf(`container_memory_working_set_bytes{namespace=%q,pod=~%q}`, b.Namespace, podMatch+".*"),
		}
		for name, q := range queries {
			samples, err := e.prom.QueryInstant(q)
			if err != nil || len(samples) == 0 {
				continue
			}
			b.Metrics = append(b.Metrics, map[string]interface{}{
				"name":    name,
				"query":   q,
				"samples": samples,
			})
		}
	}

	// 2. Loki: 대상 pod의 최근 로그
	if e.loki != nil && b.Namespace != "" {
		var logQL string
		if b.Pod != "" {
			logQL = fmt.Sprintf(`{namespace=%q,pod=%q}`, b.Namespace, b.Pod)
		} else {
			logQL = fmt.Sprintf(`{namespace=%q}`, b.Namespace)
		}
		if lines, err := e.loki.QueryRecent(logQL, e.logLines); err == nil {
			b.Logs = append(b.Logs, lines...)
		}
	}

	// 3. Kubernetes API: Events + 리소스 상태 + 노드 상태 (in-cluster, best-effort) — L2
	if e.kube != nil {
		e.kube.Enrich(b)
	}

	// 4. Rule Analyzer: 수집된 근거로 장애 유형 결정론적 1차 분류 (LLM prior) — architecture §4.3
	b.Rule = models.ClassifyRules(b)

	// 5. Runbook 매칭 (alertname + rule 카테고리) — 메타데이터/키워드 검색, LLM 컨텍스트에 주입
	cat := ""
	if b.Rule != nil {
		cat = b.Rule.Category
	}
	b.Runbooks = e.runbooks.Match(b.Alert, cat, 2)
}
