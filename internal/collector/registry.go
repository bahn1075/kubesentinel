package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// RegistryClient는 컨테이너 이미지 매니페스트를 조회하는 최소 Docker Registry v2 클라이언트다.
// v1 범위: 공개 docker.io 이미지(익명 토큰). private/기타 레지스트리는 best-effort(에러 반환).
// 목적: 이미지가 지원하는 플랫폼(arch)과 태그 존재 여부를 확인해 arch 불일치 등을 진단한다.
type RegistryClient struct {
	hc *http.Client
}

// NewRegistryClient는 기본 타임아웃을 가진 레지스트리 클라이언트를 만든다.
func NewRegistryClient() *RegistryClient {
	return &RegistryClient{hc: &http.Client{Timeout: 8 * time.Second}}
}

// ImageManifestInfo는 이미지 매니페스트 조회 결과다.
type ImageManifestInfo struct {
	Ref       string   `json:"ref"`
	Repo      string   `json:"repo"`
	Tag       string   `json:"tag"`
	TagExists bool     `json:"tag_exists"`
	Platforms []string `json:"platforms"` // 예: ["linux/amd64","linux/arm64"]
	Note      string   `json:"note,omitempty"`
}

const (
	mtOCIIndex    = "application/vnd.oci.image.index.v1+json"
	mtDockerList  = "application/vnd.docker.distribution.manifest.list.v2+json"
	mtOCIManifest = "application/vnd.oci.image.manifest.v1+json"
	mtDockerMani  = "application/vnd.docker.distribution.manifest.v2+json"
	acceptAll     = mtOCIIndex + ", " + mtDockerList + ", " + mtOCIManifest + ", " + mtDockerMani
)

// InspectImage는 이미지 참조(예: docker.io/bahn1075/helm-update:latest)의 플랫폼과 태그 존재 여부를 조회한다.
func (r *RegistryClient) InspectImage(ref string) (*ImageManifestInfo, error) {
	if r == nil {
		return nil, fmt.Errorf("registry client not configured")
	}
	repo, tag, ok := parseDockerRef(ref)
	if !ok {
		return &ImageManifestInfo{Ref: ref, Note: "unsupported registry (v1 supports public docker.io only)"}, nil
	}
	info := &ImageManifestInfo{Ref: ref, Repo: repo, Tag: tag}

	token, err := r.dockerHubToken(repo)
	if err != nil {
		return info, fmt.Errorf("auth token: %w", err)
	}

	body, mediaType, status, err := r.getManifest(repo, tag, token)
	if err != nil {
		return info, err
	}
	if status == http.StatusNotFound {
		info.TagExists = false
		info.Note = "tag not found in registry"
		return info, nil
	}
	if status != http.StatusOK {
		return info, fmt.Errorf("registry returned status %d", status)
	}
	info.TagExists = true

	platforms, err := r.platformsFrom(repo, token, body, mediaType)
	if err != nil {
		return info, err
	}
	info.Platforms = platforms
	return info, nil
}

// platformsFrom은 매니페스트(리스트/단일)에서 지원 플랫폼 목록을 추출한다("unknown"은 제외).
func (r *RegistryClient) platformsFrom(repo, token string, body []byte, mediaType string) ([]string, error) {
	// 매니페스트 리스트 / OCI 인덱스
	if strings.Contains(mediaType, "index") || strings.Contains(mediaType, "manifest.list") {
		var list struct {
			Manifests []struct {
				Platform struct {
					OS   string `json:"os"`
					Arch string `json:"architecture"`
				} `json:"platform"`
			} `json:"manifests"`
		}
		if err := json.Unmarshal(body, &list); err != nil {
			return nil, fmt.Errorf("parse manifest list: %w", err)
		}
		set := map[string]bool{}
		for _, m := range list.Manifests {
			if m.Platform.Arch == "" || m.Platform.Arch == "unknown" || m.Platform.OS == "unknown" {
				continue // attestation 등 제외
			}
			set[m.Platform.OS+"/"+m.Platform.Arch] = true
		}
		return sortedKeys(set), nil
	}
	// 단일 매니페스트(schema2/OCI) → config blob에서 arch/os
	var mani struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}
	if err := json.Unmarshal(body, &mani); err != nil || mani.Config.Digest == "" {
		return nil, fmt.Errorf("parse single manifest")
	}
	os, arch, err := r.configPlatform(repo, mani.Config.Digest, token)
	if err != nil {
		return nil, err
	}
	if arch == "" {
		return nil, nil
	}
	return []string{os + "/" + arch}, nil
}

// configPlatform은 config blob을 받아 os/architecture를 반환한다(단일 매니페스트용).
func (r *RegistryClient) configPlatform(repo, digest, token string) (string, string, error) {
	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/blobs/%s", repo, digest)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := r.hc.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("config blob status %d", resp.StatusCode)
	}
	var cfg struct {
		OS   string `json:"os"`
		Arch string `json:"architecture"`
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", "", err
	}
	return cfg.OS, cfg.Arch, nil
}

func (r *RegistryClient) getManifest(repo, ref, token string) (body []byte, mediaType string, status int, err error) {
	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", repo, ref)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", acceptAll)
	resp, err := r.hc.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return b, resp.Header.Get("Content-Type"), resp.StatusCode, nil
}

func (r *RegistryClient) dockerHubToken(repo string) (string, error) {
	url := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repo)
	resp, err := r.hc.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token status %d", resp.StatusCode)
	}
	var t struct {
		Token string `json:"token"`
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err := json.Unmarshal(b, &t); err != nil {
		return "", err
	}
	return t.Token, nil
}

// parseDockerRef는 이미지 참조를 (repo, tag)로 분해한다. docker.io(공개)만 지원하며, 아니면 ok=false.
func parseDockerRef(ref string) (repo, tag string, ok bool) {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "docker.io/")
	ref = strings.TrimPrefix(ref, "index.docker.io/")
	ref = strings.TrimPrefix(ref, "registry-1.docker.io/")

	// digest 참조(@sha256:...)
	if i := strings.Index(ref, "@"); i >= 0 {
		tag = ref[i+1:]
		ref = ref[:i]
	}
	// 태그(마지막 '/' 이후의 ':')
	if tag == "" {
		if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
			tag = ref[i+1:]
			ref = ref[:i]
		} else {
			tag = "latest"
		}
	}
	// 첫 세그먼트가 레지스트리 호스트('.' 또는 ':')면 docker.io가 아니므로 미지원
	first := ref
	if i := strings.Index(ref, "/"); i >= 0 {
		first = ref[:i]
	}
	if strings.ContainsAny(first, ".:") {
		return "", "", false
	}
	// 공식 이미지(단일 세그먼트)는 library/ 접두
	if !strings.Contains(ref, "/") {
		ref = "library/" + ref
	}
	return ref, tag, true
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
