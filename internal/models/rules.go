package models

import "strings"

// RuleResult는 LLM 이전 단계의 결정론적(코드) 장애 분류 결과입니다. (architecture.md §4.3 Rule Analyzer)
type RuleResult struct {
	Category  string   `json:"category"`            // CrashLoopBackOff | OOMKilled | ... | Unknown
	Rationale string   `json:"rationale,omitempty"` // 어떤 신호로 분류했는지
	Signals   []string `json:"signals,omitempty"`   // 매칭된 신호들
}

// ClassifyRules는 alert명·Events·리소스 상태의 신호로 장애 유형을 1차 분류한다(휴리스틱).
// LLM에 강한 prior로 전달되고 인시던트에 표시된다. 확실치 않으면 Unknown.
func ClassifyRules(b *EvidenceBundle) *RuleResult {
	if b == nil {
		return nil
	}
	parts := []string{b.Alert}
	parts = append(parts, b.Events...)
	parts = append(parts, containerStrings(b.ResourceYAML)...)
	hay := strings.ToLower(strings.Join(parts, " | "))

	// (우선순위 순) 카테고리 → 매칭 키워드
	rules := []struct {
		cat  string
		keys []string
	}{
		{"OOMKilled", []string{"oomkill", "oom_kill", "oomkilled", "out of memory"}},
		{"CrashLoopBackOff", []string{"crashloop", "crash loop", "back-off restarting", "backoff restarting"}},
		{"ImagePullBackOff", []string{"imagepullbackoff", "errimagepull", "errimage", "failed to pull image"}},
		{"ConfigError", []string{"createcontainerconfigerror", "createcontainerconfig"}},
		{"JobFailed", []string{"backofflimitexceeded", "job has reached", "jobfailed", "deadlineexceeded"}},
		{"Unschedulable", []string{"failedscheduling", "unschedulable", "insufficient cpu", "insufficient memory"}},
		{"VolumeIssue", []string{"failedmount", "failedattachvolume", "volumemismatch"}},
		{"ProbeFailure", []string{"liveness probe failed", "readiness probe failed", "unhealthy", "probe failed"}},
		{"ControlPlaneDown", []string{"kubescheduler", "controllermanager", "kubeapiserver", "apiserverdown", "etcd"}},
		{"NodeIssue", []string{"node not ready", "notready", "memorypressure", "diskpressure", "kubelet"}},
		{"TargetDown", []string{"targetdown"}},
	}

	cat, rationale := "Unknown", ""
	signals := []string{}
	for _, r := range rules {
		for _, k := range r.keys {
			if strings.Contains(hay, k) {
				if cat == "Unknown" {
					cat, rationale = r.cat, "matched signal: "+k
				}
				signals = append(signals, r.cat+"←"+k)
				break // rule당 키 1개
			}
		}
	}
	return &RuleResult{Category: cat, Rationale: rationale, Signals: signals}
}

// containerStrings는 ResourceYAML["containers"]가 []string이면 그대로 반환한다(Pod 상태의 waiting/restart 요약).
func containerStrings(rs map[string]interface{}) []string {
	if rs == nil {
		return nil
	}
	if cs, ok := rs["containers"].([]string); ok {
		return cs
	}
	return nil
}
