package notifier

import (
	"kubesentinel-ai/internal/models"
)

// Notifier는 진단 결과를 외부 채널로 알리는 인터페이스입니다. (architecture.md §4.7)
// Discord / Slack / Teams webhook을 동일 인터페이스로 추상화한다.
type Notifier interface {
	// NotifyDiagnosis는 RCA 결과 + 근거를 알림 채널로 전송합니다. (MVP-0: 읽기 전용 RCA + 알림)
	NotifyDiagnosis(bundle *models.EvidenceBundle, result *models.DiagnosisResult) error
}

// noopNotifier는 알림 채널이 설정되지 않았을 때 사용하는 무동작 구현입니다.
type noopNotifier struct{}

func (noopNotifier) NotifyDiagnosis(*models.EvidenceBundle, *models.DiagnosisResult) error {
	return nil
}
