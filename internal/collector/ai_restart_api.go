package collector

import (
	"fmt"
	"net/http"
	"os"
)

// handleAIRestart는 이 앱 자신의 Deployment에 rollout-restart를 트리거한다.
// AI 설정(endpoint/model)은 프로세스 기동 시에만 로드되므로, DB에 저장한 값을 반영하려면 재시작이 필요하다.
// POST /api/ai/restart. RBAC(patch on apps/deployments)이 이 Deployment 1개로 scope되어 있지 않으면 실패한다
// (rbac-restart.yaml, values.yaml rbac.selfRestart.enabled=false가 기본값 — 명시적으로 켜야 동작).
func (s *WebhookServer) handleAIRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ns := os.Getenv("POD_NAMESPACE")
	name := os.Getenv("KUBESENTINEL_AI_DEPLOYMENT_NAME")
	if ns == "" || name == "" {
		http.Error(w, "재시작 비활성: POD_NAMESPACE/KUBESENTINEL_AI_DEPLOYMENT_NAME env가 설정되지 않았습니다", http.StatusServiceUnavailable)
		return
	}
	kube := NewKubeCollector()
	if kube == nil {
		http.Error(w, "재시작 실패: in-cluster Kubernetes API에 연결할 수 없습니다", http.StatusServiceUnavailable)
		return
	}
	if err := kube.RestartDeployment(ns, name); err != nil {
		http.Error(w, fmt.Sprintf("재시작 실패: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarting"})
}
