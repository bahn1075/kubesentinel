package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kubesentinel-ai/internal/config"
	"kubesentinel-ai/internal/models"
)

// webhookNotifier는 Discord/Slack/Teams의 incoming webhook으로 텍스트 메시지를 보냅니다.
// 세 채널 모두 JSON 한 필드(content/text)에 메시지를 담는 단순 webhook을 지원하므로
// payloadKey만 다르게 두고 동일 구현을 공유한다.
type webhookNotifier struct {
	url        string
	payloadKey string // discord: "content", slack/teams: "text"
	grafanaURL string
	client     *http.Client
}

// New는 NotifierConfig를 기반으로 적절한 Notifier를 생성합니다.
// webhook URL이 비어 있으면 무동작(noop) Notifier를 반환한다.
func New(cfg config.NotifierConfig, grafanaURL string) (Notifier, error) {
	if cfg.Webhook == "" {
		return noopNotifier{}, nil
	}

	var payloadKey string
	switch strings.ToLower(cfg.Type) {
	case "discord":
		payloadKey = "content"
	case "slack", "teams", "": // teams legacy connector·slack 모두 "text"
		payloadKey = "text"
	default:
		return nil, fmt.Errorf("unsupported notifier type: %q (discord|slack|teams)", cfg.Type)
	}

	return &webhookNotifier{
		url:        cfg.Webhook,
		payloadKey: payloadKey,
		grafanaURL: grafanaURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (n *webhookNotifier) NotifyDiagnosis(bundle *models.EvidenceBundle, result *models.DiagnosisResult) error {
	msg := n.formatMessage(bundle, result)

	payload := map[string]string{n.payloadKey: msg}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	resp, err := n.client.Post(n.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	// 2xx 외에는 실패로 간주 (webhook 별 성공 코드: Discord 204, Slack 200)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification webhook returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// formatMessage는 진단 결과를 사람이 읽기 좋은 알림 메시지로 만듭니다.
func (n *webhookNotifier) formatMessage(bundle *models.EvidenceBundle, result *models.DiagnosisResult) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "🚨 **KubeSentinel — Incident %s**\n", bundle.IncidentID)
	fmt.Fprintf(&sb, "• Alert: `%s`", bundle.Alert)
	if bundle.Severity != "" {
		fmt.Fprintf(&sb, " (severity: %s)", bundle.Severity)
	}
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "• Target: `%s/%s`", bundle.Namespace, bundle.Workload)
	if bundle.Pod != "" {
		fmt.Fprintf(&sb, " (pod: %s)", bundle.Pod)
	}
	sb.WriteString("\n\n")

	fmt.Fprintf(&sb, "**Root Cause**\n%s\n\n", fallback(result.RootCause, "(분석 결과 없음)"))
	fmt.Fprintf(&sb, "**Summary**\n%s\n", fallback(result.Summary, "(요약 없음)"))
	if result.Confidence > 0 {
		fmt.Fprintf(&sb, "_confidence: %.2f_\n", result.Confidence)
	}

	if len(result.ProposedActions) > 0 {
		sb.WriteString("\n**Proposed Actions** (제안일 뿐, 적용은 정책·승인 후)\n")
		for i, a := range result.ProposedActions {
			fmt.Fprintf(&sb, "%d. [%s · risk=%s] %s", i+1, fallback(a.Type, "suggestion"), fallback(a.Risk, "?"), a.Description)
			if a.Target != "" {
				fmt.Fprintf(&sb, " → `%s`", a.Target)
			}
			sb.WriteString("\n")
		}
	}

	// 딥링크 (architecture.md §4.7)
	if n.grafanaURL != "" {
		fmt.Fprintf(&sb, "\n🔗 Grafana: %s\n", n.grafanaURL)
	}

	return sb.String()
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
