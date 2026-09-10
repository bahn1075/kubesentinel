package collector

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"kubesentinel-ai/internal/models"
)

// probe는 룰 분류 카테고리에 따라 결정론적 심층 조사를 수행해 b.ProbeFindings에 구조화된 근거를 추가한다.
// 코드가 상관(correlation)을 대신 수행하므로 작은 LLM에서도 구체적 진단이 가능하다. best-effort.
func (e *Enricher) probe(b *models.EvidenceBundle) {
	if b == nil || b.Rule == nil {
		return
	}
	switch b.Rule.Category {
	case "ImagePullBackOff":
		b.ProbeFindings = append(b.ProbeFindings, e.probeImagePull(b)...)
	case "CrashLoopBackOff":
		b.ProbeFindings = append(b.ProbeFindings, e.probeCrashLoop(b)...)
	case "OOMKilled":
		b.ProbeFindings = append(b.ProbeFindings, e.probeOOM(b)...)
	case "Unschedulable":
		b.ProbeFindings = append(b.ProbeFindings, e.probeUnschedulable(b)...)
	}
}

// probeCrashLoop은 컨테이너의 종료코드/사유를 뽑아 재시작 원인을 구체화한다.
func (e *Enricher) probeCrashLoop(b *models.EvidenceBundle) []string {
	if e.kube == nil || b.Pod == "" {
		return nil
	}
	var out []string
	for _, c := range e.kube.PodContainers(b.Namespace, b.Pod) {
		if f := crashLoopFinding(c); f != "" {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil
	}
	// 로그가 이미 수집돼 있으면 종료 직전 로그를 함께 보라고 안내
	if len(b.Logs) > 0 {
		out = append(out, "probe(CrashLoop): 위 종료코드와 함께 수집된 최근 로그(logs)를 대조해 근본 원인을 특정하세요.")
	}
	return out
}

// probeOOM은 OOMKilled 컨테이너의 메모리 limit을 근거로 상향/누수 조치를 제시한다.
func (e *Enricher) probeOOM(b *models.EvidenceBundle) []string {
	if e.kube == nil || b.Pod == "" {
		return nil
	}
	var out []string
	for _, c := range e.kube.PodContainers(b.Namespace, b.Pod) {
		if c.TerminatedReason == "OOMKilled" || c.LastTerminatedReason == "OOMKilled" {
			out = append(out, fmt.Sprintf(
				"probe(OOM): ⚠️ 컨테이너 %s OOMKilled (재시작 %d회). memory limit=%s, request=%s. 조치: 워킹셋이 지속 초과면 limit 상향, 급증/누수면 앱 메모리 프로파일링.",
				c.Name, c.RestartCount, orNone(c.MemLimit), orNone(c.MemRequest)))
		}
	}
	return out
}

// probeUnschedulable은 스케줄러의 FailedScheduling 사유(결정론적)와 노드 arch/taint를 함께 제시한다.
func (e *Enricher) probeUnschedulable(b *models.EvidenceBundle) []string {
	var out []string
	for _, ev := range b.Events {
		if strings.Contains(ev, "FailedScheduling") || strings.Contains(strings.ToLower(ev), "unschedulable") {
			out = append(out, "probe(Unschedulable): 스케줄러 사유 — "+strings.TrimSpace(ev))
			break
		}
	}
	if e.kube != nil {
		if info := e.kube.NodeInfoSummary(); len(info) > 0 {
			out = append(out, "probe(Unschedulable): 노드 현황 — "+info[0]) // 첫 줄 = arch 요약
		}
	}
	if len(out) > 0 {
		out = append(out, "probe(Unschedulable): 위 사유에 따라 조치 — 리소스 부족이면 요청량 하향/노드 증설, taint면 toleration 추가, nodeSelector/affinity면 라벨 확인.")
	}
	return out
}

// crashLoopFinding은 컨테이너 진단에서 재시작/종료 원인 한 줄을 만든다(해당 없으면 "").
func crashLoopFinding(c ContainerDiag) string {
	reason := c.TerminatedReason
	code := c.TerminatedExitCode
	if reason == "" && c.LastTerminatedReason != "" {
		reason, code = c.LastTerminatedReason, c.LastTerminatedExitCode
	}
	if reason == "" && c.RestartCount == 0 {
		return ""
	}
	hint := ""
	switch {
	case code == 137 || reason == "OOMKilled":
		hint = " (137/OOM → 메모리 부족 가능: limit 상향/누수 점검)"
	case code == 143:
		hint = " (143 → SIGTERM 종료)"
	case code == 1 || code == 2:
		hint = " (앱 오류: 설정/시크릿/환경변수 또는 코드 예외 — 직전 로그 확인)"
	case code == 127:
		hint = " (127 → 명령/바이너리 없음: entrypoint/이미지 확인)"
	}
	return fmt.Sprintf("probe(CrashLoop): 컨테이너 %s 재시작 %d회, 직전 종료 reason=%s exitCode=%d%s",
		c.Name, c.RestartCount, orNone(reason), code, hint)
}

func orNone(s string) string {
	if s == "" || s == "0" {
		return "(none)"
	}
	return s
}

var rePullImage = regexp.MustCompile(`(?:pulling image|image)\s+"([^"]+)"`)

// probeImagePull은 대상 이미지의 레지스트리 플랫폼과 노드 아키텍처를 비교해 근본 원인을 짚는다.
func (e *Enricher) probeImagePull(b *models.EvidenceBundle) []string {
	if e.registry == nil {
		return nil
	}
	images := e.imagesForBundle(b)
	if len(images) == 0 {
		return []string{"probe(ImagePull): 대상 이미지를 확인하지 못했습니다(파드 스펙/이벤트에서 이미지 미검출)."}
	}

	var nodeArchs []string
	if e.kube != nil {
		nodeArchs = e.kube.NodeArchs()
	}

	var out []string
	for _, img := range images {
		info, err := e.registry.InspectImage(img)
		if err != nil {
			out = append(out, fmt.Sprintf("probe(ImagePull): 이미지 %s 조회 실패: %v", img, err))
			continue
		}
		if info.Note != "" && len(info.Platforms) == 0 && !info.TagExists {
			// 태그 없음 또는 미지원 레지스트리
			if strings.Contains(info.Note, "tag not found") {
				out = append(out, fmt.Sprintf("probe(ImagePull): ⚠️ 이미지 태그 없음 — %s 가 레지스트리에 존재하지 않습니다. 조치: 태그 오타/누락 수정(매니페스트의 image 참조 확인).", img))
			} else {
				out = append(out, fmt.Sprintf("probe(ImagePull): 이미지 %s — %s (arch 확인 불가).", img, info.Note))
			}
			continue
		}
		if len(info.Platforms) == 0 {
			out = append(out, fmt.Sprintf("probe(ImagePull): 이미지 %s 의 플랫폼을 확인하지 못했습니다.", img))
			continue
		}
		imgArchs := archSet(info.Platforms)
		if len(nodeArchs) > 0 && !intersects(imgArchs, nodeArchs) {
			out = append(out, fmt.Sprintf(
				"probe(ImagePull): ⚠️ 아키텍처 불일치 — 이미지 %s 는 [%s]만 지원하나 클러스터 노드는 [%s]. 스케줄된 노드에서 pull 불가(매니페스트에 노드 arch 없음). 조치: 이미지를 멀티아치(예: linux/amd64,linux/arm64)로 재빌드하거나 노드 arch와 일치하는 태그를 사용.",
				img, strings.Join(info.Platforms, ","), strings.Join(nodeArchs, ",")))
		} else {
			out = append(out, fmt.Sprintf(
				"probe(ImagePull): 이미지 %s platforms=[%s], 노드 arch=[%s] 일치 → arch 문제 아님. 인증(imagePullSecret)/네트워크/레지스트리 레이트리밋을 확인.",
				img, strings.Join(info.Platforms, ","), strings.Join(nodeArchs, ",")))
		}
	}
	return out
}

// imagesForBundle은 대상 이미지를 파드 스펙(우선) 또는 이벤트 메시지에서 추출한다.
func (e *Enricher) imagesForBundle(b *models.EvidenceBundle) []string {
	if e.kube != nil && b.Pod != "" {
		if imgs := e.kube.PodImages(b.Namespace, b.Pod); len(imgs) > 0 {
			return imgs
		}
	}
	// 이벤트 메시지에서 pull 대상 이미지 파싱 (예: Back-off pulling image "docker.io/x:tag")
	seen := map[string]bool{}
	var out []string
	for _, ev := range b.Events {
		for _, m := range rePullImage.FindAllStringSubmatch(ev, -1) {
			img := m[1]
			if img != "" && !seen[img] {
				seen[img] = true
				out = append(out, img)
			}
		}
	}
	return out
}

// archSet은 "os/arch" 목록에서 arch 집합을 만든다(예: "linux/amd64" → "amd64").
func archSet(platforms []string) []string {
	set := map[string]bool{}
	for _, p := range platforms {
		a := p
		if i := strings.LastIndex(p, "/"); i >= 0 {
			a = p[i+1:]
		}
		set[a] = true
	}
	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// intersects는 두 문자열 슬라이스가 공통 원소를 갖는지 판정한다.
func intersects(a, b []string) bool {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	for _, y := range b {
		if set[y] {
			return true
		}
	}
	return false
}
