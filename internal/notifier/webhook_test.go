package notifier

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kubesentinel-ai/internal/config"
	"kubesentinel-ai/internal/models"
)

func TestNew_EmptyWebhookReturnsNoop(t *testing.T) {
	n, err := New(config.NotifierConfig{Type: "slack"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := n.(noopNotifier); !ok {
		t.Errorf("expected noopNotifier when webhook empty, got %T", n)
	}
}

func TestNew_UnsupportedType(t *testing.T) {
	if _, err := New(config.NotifierConfig{Type: "carrier-pigeon", Webhook: "http://x"}, ""); err == nil {
		t.Error("expected error for unsupported notifier type")
	}
}

func TestNotifyDiagnosis_PayloadKeyAndContent(t *testing.T) {
	cases := []struct {
		typ     string
		wantKey string
	}{
		{"discord", "content"},
		{"slack", "text"},
		{"teams", "text"},
	}

	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			var body map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &body)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			n, err := New(config.NotifierConfig{Type: tc.typ, Webhook: srv.URL}, "http://grafana")
			if err != nil {
				t.Fatalf("New error: %v", err)
			}

			bundle := &models.EvidenceBundle{IncidentID: "inc-1", Alert: "KubePodCrashLooping", Namespace: "ns", Workload: "app", Pod: "app-1", Severity: "critical"}
			result := &models.DiagnosisResult{
				RootCause:       "OOM",
				Summary:         "memory limit too low",
				ProposedActions: []models.ProposedAction{{Type: "git_pr", Description: "raise limit", Target: "values.yaml", Risk: "medium"}},
			}

			if err := n.NotifyDiagnosis(bundle, result); err != nil {
				t.Fatalf("NotifyDiagnosis error: %v", err)
			}

			msg, ok := body[tc.wantKey]
			if !ok {
				t.Fatalf("payload missing key %q; got keys %v", tc.wantKey, keys(body))
			}
			for _, want := range []string{"inc-1", "KubePodCrashLooping", "OOM", "raise limit", "grafana"} {
				if !strings.Contains(msg, want) {
					t.Errorf("message missing %q\n---\n%s", want, msg)
				}
			}
		})
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
