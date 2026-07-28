package collector

import (
	"reflect"
	"testing"
)

func TestParseDockerRef(t *testing.T) {
	cases := []struct {
		in       string
		wantRepo string
		wantTag  string
		wantOK   bool
	}{
		{"docker.io/bahn1075/helm-update:latest", "bahn1075/helm-update", "latest", true},
		{"bahn1075/helm-update:aarch64", "bahn1075/helm-update", "aarch64", true},
		{"bahn1075/helm-update", "bahn1075/helm-update", "latest", true},
		{"nginx", "library/nginx", "latest", true},
		{"nginx:1.27", "library/nginx", "1.27", true},
		{"ghcr.io/org/app:1.0", "", "", false},       // 비-docker.io 미지원
		{"registry.example.com:5000/x:1", "", "", false}, // 포트 있는 호스트
	}
	for _, c := range cases {
		repo, tag, ok := parseDockerRef(c.in)
		if ok != c.wantOK || (ok && (repo != c.wantRepo || tag != c.wantTag)) {
			t.Errorf("parseDockerRef(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, repo, tag, ok, c.wantRepo, c.wantTag, c.wantOK)
		}
	}
}

func TestArchSet(t *testing.T) {
	got := archSet([]string{"linux/amd64", "linux/arm64", "windows/amd64"})
	want := []string{"amd64", "arm64"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("archSet = %v, want %v", got, want)
	}
}

func TestIntersects(t *testing.T) {
	// arch 불일치: 이미지 amd64 전용, 노드 arm64 → 교집합 없음
	if intersects([]string{"amd64"}, []string{"arm64"}) {
		t.Error("expected no intersection for amd64 image vs arm64 nodes")
	}
	// 멀티아치 이미지 → arm64 노드와 교집합 있음
	if !intersects([]string{"amd64", "arm64"}, []string{"arm64"}) {
		t.Error("expected intersection for multi-arch image vs arm64 nodes")
	}
}
