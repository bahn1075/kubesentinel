package collector

import (
	"encoding/json"
	"fmt"
	"strings"

	"kubesentinel-ai/internal/config"
	"kubesentinel-ai/internal/diagnosis"
)

// ToolRunner는 agentic 진단에서 LLM이 요청하는 read-only 근거 수집 도구를 실행합니다.
// diagnosis.ToolRunner 인터페이스를 구현하며 main에서 Engine에 주입된다.
type ToolRunner struct {
	prom *PrometheusClient
	loki *LokiClient
	kube *KubeCollector
}

// NewToolRunner는 설정 기반으로 도구 실행기를 만든다. (in-cluster/엔드포인트 없으면 해당 도구는 비활성)
func NewToolRunner(cfg config.CollectorConfig) *ToolRunner {
	return &ToolRunner{
		prom: NewPrometheusClient(cfg.PrometheusURL),
		loki: NewLokiClient(cfg.LokiURL),
		kube: NewKubeCollector(),
	}
}

// Specs는 LLM에 노출할 도구 목록을 반환합니다.
func (t *ToolRunner) Specs() []diagnosis.ToolSpec {
	return []diagnosis.ToolSpec{
		{Name: "prometheus_query", Description: "Run an instant PromQL query", Args: `{"query":"<PromQL>"}`},
		{Name: "loki_query", Description: "Fetch recent logs by LogQL selector", Args: `{"query":"{namespace=\"x\",pod=\"y\"}","limit":50}`},
		{Name: "k8s_events", Description: "Recent Kubernetes events in a namespace (optionally for one object)", Args: `{"namespace":"x","name":"<optional>"}`},
		{Name: "k8s_list_pods", Description: "List pods and their status/restarts in a namespace", Args: `{"namespace":"x"}`},
	}
}

// Run은 도구를 실행하고 결과 문자열을 반환한다(실패는 "ERROR: ..."로 문자열 반환 → LLM이 인지).
func (t *ToolRunner) Run(name string, args map[string]interface{}) string {
	switch name {
	case "prometheus_query":
		if t.prom == nil {
			return "ERROR: prometheus not configured"
		}
		q := argStr(args, "query")
		if q == "" {
			return "ERROR: missing 'query'"
		}
		samples, err := t.prom.QueryInstant(q)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		b, _ := json.Marshal(samples)
		return string(b)

	case "loki_query":
		if t.loki == nil {
			return "ERROR: loki not configured"
		}
		q := argStr(args, "query")
		if q == "" {
			return "ERROR: missing 'query'"
		}
		limit := 50
		if v, ok := args["limit"].(float64); ok && v > 0 {
			limit = int(v)
		}
		lines, err := t.loki.QueryRecent(q, limit)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		if len(lines) == 0 {
			return "(no log lines)"
		}
		return strings.Join(lines, "\n")

	case "k8s_events":
		if t.kube == nil {
			return "ERROR: kubernetes API not available (not in-cluster)"
		}
		ns := argStr(args, "namespace")
		if ns == "" {
			return "ERROR: missing 'namespace'"
		}
		ev := t.kube.Events(ns, argStr(args, "name"))
		if len(ev) == 0 {
			return "(no events)"
		}
		return strings.Join(ev, "\n")

	case "k8s_list_pods":
		if t.kube == nil {
			return "ERROR: kubernetes API not available (not in-cluster)"
		}
		ns := argStr(args, "namespace")
		if ns == "" {
			return "ERROR: missing 'namespace'"
		}
		pods := t.kube.ListPods(ns)
		if len(pods) == 0 {
			return "(no pods)"
		}
		return strings.Join(pods, "\n")

	default:
		return fmt.Sprintf("ERROR: unknown tool %q", name)
	}
}

func argStr(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}
