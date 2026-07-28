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
	}
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
