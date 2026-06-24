package models

import "testing"

func TestNewEvidenceBundle(t *testing.T) {
	payload := AlertmanagerPayload{
		Receiver: "kubesentinel",
		Alerts: []Alert{
			{
				Status: "firing",
				Labels: Labels{
					"alertname":  "KubePodCrashLooping",
					"namespace":  "production",
					"pod":        "api-server-abc123",
					"deployment": "api-server",
					"severity":   "critical",
				},
				Annotations: Labels{"summary": "pod is crash looping"},
			},
		},
	}

	b := NewEvidenceBundle(payload)
	if b == nil {
		t.Fatal("expected non-nil bundle")
	}
	if b.Alert != "KubePodCrashLooping" {
		t.Errorf("alert = %q, want KubePodCrashLooping", b.Alert)
	}
	if b.Namespace != "production" {
		t.Errorf("namespace = %q, want production", b.Namespace)
	}
	// deployment 라벨이 pod보다 우선해 워크로드로 선택되어야 한다.
	if b.Workload != "api-server" {
		t.Errorf("workload = %q, want api-server", b.Workload)
	}
	if b.Pod != "api-server-abc123" {
		t.Errorf("pod = %q, want api-server-abc123", b.Pod)
	}
	if b.Severity != "critical" {
		t.Errorf("severity = %q, want critical", b.Severity)
	}
	if b.Source != "kubesentinel" {
		t.Errorf("source = %q, want kubesentinel", b.Source)
	}
	// 비-nil 슬라이스/맵으로 초기화되어 JSON 직렬화 시 null이 아니어야 한다.
	if b.Metrics == nil || b.Logs == nil || b.Events == nil || b.ResourceYAML == nil {
		t.Error("collections should be initialized non-nil")
	}
}

func TestNewEvidenceBundle_NoAlerts(t *testing.T) {
	if b := NewEvidenceBundle(AlertmanagerPayload{}); b != nil {
		t.Errorf("expected nil bundle for empty payload, got %+v", b)
	}
}

func TestNewEvidenceBundle_WorkloadFallsBackToPod(t *testing.T) {
	payload := AlertmanagerPayload{
		Alerts: []Alert{{Labels: Labels{"alertname": "X", "namespace": "ns", "pod": "lonely-pod"}}},
	}
	b := NewEvidenceBundle(payload)
	if b.Workload != "lonely-pod" {
		t.Errorf("workload = %q, want lonely-pod (fallback)", b.Workload)
	}
	if b.Source != "alertmanager" {
		t.Errorf("source = %q, want alertmanager (default)", b.Source)
	}
}
