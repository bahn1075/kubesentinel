package collector

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kubesentinel-ai/internal/models"
)

// processBundle은 한 incident에 대한 공용 처리 경로입니다 (webhook·poller 공용):
// 근거 보강 → AI 분석 → 영속화 → 알림.
func (s *WebhookServer) processBundle(b *models.EvidenceBundle) {
	fmt.Printf("\n[KubeSentinel] 🔍 Analyzing Incident: %s\n", b.IncidentID)

	if s.Enricher != nil {
		s.Enricher.Enrich(b)
	}

	result, err := s.Engine.Analyze(b)
	state := "DiagnosisCompleted"
	if err != nil {
		fmt.Printf("[KubeSentinel] ❌ Analysis Failed: %v\n", err)
		state = "ValidationFailed"
		result = nil
	} else {
		fmt.Printf("[KubeSentinel] ✅ Analysis Complete! Root Cause: %s\n", result.RootCause)
	}

	if s.Store != nil {
		if e := s.Store.SaveIncident(models.NewIncidentView(b, result, state)); e != nil {
			fmt.Printf("[KubeSentinel] ⚠️  Save Incident Failed: %v\n", e)
		}
	}

	if result != nil && s.Notifier != nil {
		if err := s.Notifier.NotifyDiagnosis(b, result); err != nil {
			fmt.Printf("[KubeSentinel] ⚠️  Notification Failed: %v\n", err)
		}
	}
}

// amV2Alert는 Alertmanager v2 API(GET /api/v2/alerts)의 alert 항목입니다.
type amV2Alert struct {
	Labels       models.Labels `json:"labels"`
	Annotations  models.Labels `json:"annotations"`
	StartsAt     time.Time     `json:"startsAt"`
	EndsAt       time.Time     `json:"endsAt"`
	GeneratorURL string        `json:"generatorURL"`
	Fingerprint  string        `json:"fingerprint"`
	Status       struct {
		State string `json:"state"` // active | suppressed | unprocessed
	} `json:"status"`
}

// StartAlertmanagerPoller는 Alertmanager API를 주기적으로 폴링해 firing alert를 처리합니다.
// prometheus/alertmanager 설정 변경이 전혀 필요 없는 pull 방식.
// fingerprint로 중복을 제거하고, alert가 사라지면(resolved) 추적에서 제거해 재발화 시 재처리한다.
func (s *WebhookServer) StartAlertmanagerPoller() {
	interval := time.Duration(s.PollIntervalSec) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	seen := map[string]bool{} // fingerprint → 처리됨

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		alerts, err := fetchActiveAlerts(s.AlertmanagerURL)
		if err != nil {
			fmt.Printf("[KubeSentinel] ⚠️  Alertmanager poll failed: %v\n", err)
			<-ticker.C
			continue
		}

		current := map[string]bool{}
		for _, a := range alerts {
			current[a.Fingerprint] = true
			if a.Status.State != "active" {
				continue // silenced/inhibited 제외
			}
			sev := a.Labels["severity"]
			if a.Labels["alertname"] == "Watchdog" || (sev != "warning" && sev != "critical") {
				continue // 노이즈 제외 (warning/critical만)
			}
			if seen[a.Fingerprint] {
				continue // 이미 처리한 firing alert
			}
			seen[a.Fingerprint] = true
			bundle := models.NewEvidenceBundle(models.AlertmanagerPayload{
				Receiver: "alertmanager-poll",
				Status:   "firing",
				Alerts: []models.Alert{{
					Status:       "firing",
					Labels:       a.Labels,
					Annotations:  a.Annotations,
					StartsAt:     a.StartsAt,
					GeneratorURL: a.GeneratorURL,
					Fingerprint:  a.Fingerprint,
				}},
			})
			if bundle != nil {
				bundle.RelatedAlerts = relatedFromV2(alerts, a.Fingerprint)
				go s.processBundle(bundle)
			}
		}
		// resolved 정리: 더 이상 active 목록에 없는 fingerprint는 추적 해제(재발화 시 재처리)
		for fp := range seen {
			if !current[fp] {
				delete(seen, fp)
			}
		}
		<-ticker.C
	}
}

// relatedFromV2는 자신(selfFingerprint)을 제외한 동시 발생 alert들을 상관 컨텍스트로 매핑한다.
func relatedFromV2(alerts []amV2Alert, selfFingerprint string) []models.RelatedAlert {
	out := make([]models.RelatedAlert, 0, len(alerts))
	for _, a := range alerts {
		if a.Fingerprint == selfFingerprint {
			continue
		}
		out = append(out, models.RelatedAlert{
			Alertname: a.Labels["alertname"],
			Namespace: a.Labels["namespace"],
			Severity:  a.Labels["severity"],
			Summary:   a.Annotations["summary"],
		})
	}
	return out
}

// relatedFromAlerts는 webhook 페이로드의 alert 슬라이스를 상관 컨텍스트로 매핑한다.
func relatedFromAlerts(alerts []models.Alert) []models.RelatedAlert {
	out := make([]models.RelatedAlert, 0, len(alerts))
	for _, a := range alerts {
		out = append(out, models.RelatedAlert{
			Alertname: a.Labels["alertname"],
			Namespace: a.Labels["namespace"],
			Severity:  a.Labels["severity"],
			Summary:   a.Annotations["summary"],
		})
	}
	return out
}

// fetchActiveAlerts는 Alertmanager v2 API에서 활성 alert 목록을 가져옵니다.
func fetchActiveAlerts(base string) ([]amV2Alert, error) {
	u := strings.TrimRight(base, "/") + "/api/v2/alerts?" + url.Values{
		"active":    {"true"},
		"silenced":  {"false"},
		"inhibited": {"false"},
	}.Encode()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("get alerts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alertmanager status %d", resp.StatusCode)
	}
	var alerts []amV2Alert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return nil, fmt.Errorf("decode alerts: %w", err)
	}
	return alerts, nil
}
