package models

import "testing"

// EvidenceBundleFromView가 NewIncidentView로 저장한 근거를 재분석용 EvidenceBundle로
// 손실 없이(핵심 진단 입력 기준) 복원하는지 검증한다("재분석 실행" 버튼의 백엔드 경로).
func TestEvidenceBundleFromView_RoundTrip(t *testing.T) {
	original := &EvidenceBundle{
		IncidentID: "inc-1",
		Alert:      "KubeJobFailed",
		Namespace:  "default",
		Workload:   "kube-state-metrics",
		Pod:        "kube-state-metrics-abc",
		Severity:   "warning",
		Metrics:    []map[string]interface{}{{"name": "up"}},
		Logs:       []string{"log line 1"},
		Events:     []string{"event 1"},
		RelatedAlerts: []RelatedAlert{
			{Alertname: "Watchdog", Namespace: "monitoring"},
		},
		Runbooks: []RunbookMatch{
			{Title: "JobFailed runbook", Category: "JobFailed", Body: "## Steps\ndo x"},
		},
		ProbeFindings: []string{"probe finding"},
		GitContext:    GitContext{Repo: "org/repo", Path: "k8s/app", LastCommit: "abc123"},
	}

	view := NewIncidentView(original, nil, "ValidationFailed")
	restored := EvidenceBundleFromView(view)

	if restored.IncidentID != original.IncidentID || restored.Alert != original.Alert ||
		restored.Namespace != original.Namespace || restored.Workload != original.Workload ||
		restored.Pod != original.Pod || restored.Severity != original.Severity {
		t.Fatalf("identity fields mismatch: %+v vs %+v", restored, original)
	}
	if len(restored.Metrics) != 1 || len(restored.Logs) != 1 || len(restored.Events) != 1 {
		t.Fatalf("evidence slices not restored: %+v", restored)
	}
	if len(restored.RelatedAlerts) != 1 || restored.RelatedAlerts[0].Alertname != "Watchdog" {
		t.Fatalf("related alerts not restored: %+v", restored.RelatedAlerts)
	}
	if len(restored.Runbooks) != 1 || restored.Runbooks[0].Body != "## Steps\ndo x" {
		t.Fatalf("runbooks not restored: %+v", restored.Runbooks)
	}
	if len(restored.ProbeFindings) != 1 {
		t.Fatalf("probe findings not restored: %+v", restored.ProbeFindings)
	}
	if restored.GitContext != original.GitContext {
		t.Fatalf("git context not restored: %+v vs %+v", restored.GitContext, original.GitContext)
	}
}
