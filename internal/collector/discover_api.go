package collector

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DiscoveredEndpoint는 자동조회로 찾은(또는 못 찾은) 관측 스택 구성요소 하나입니다.
type DiscoveredEndpoint struct {
	Key         string   `json:"key"`                   // prometheus | loki | alertmanager | grafana
	Label       string   `json:"label"`                 // 화면 표시명
	URL         string   `json:"url,omitempty"`         // 채택된 in-cluster 주소
	Service     string   `json:"service,omitempty"`     // <namespace>/<name>:<port>
	Candidates  []string `json:"candidates,omitempty"`  // 동일 역할로 보이는 다른 후보 주소
	InstallHint string   `json:"installHint,omitempty"` // 미검출 시 설치 안내
}

// DiscoverResult는 GET /api/collector/discover 응답입니다.
type DiscoverResult struct {
	Found     []DiscoveredEndpoint `json:"found"`
	Missing   []DiscoveredEndpoint `json:"missing"`
	Scanned   int                  `json:"scannedServices"`
	Available bool                 `json:"available"` // in-cluster API 접근 가능 여부
	Error     string               `json:"error,omitempty"`
}

// svcRule은 한 구성요소를 서비스 목록에서 식별하는 규칙입니다.
// nameHints는 앞쪽이 더 높은 우선순위(정확한 이름 → 느슨한 포함)로 정렬한다.
type svcRule struct {
	key         string
	label       string
	nameHints   []string // 서비스명에 포함되면 후보 (앞이 우선)
	portNames   []string // 포트 이름 매칭 (http-web, http-metrics 등)
	portNumbers []int32  // 포트 번호 매칭 (기본 포트)
	exclude     []string // 이 문자열을 포함하면 제외 (operated 대상 외 사이드카 등)
	installHint string
}

// 관측 스택 표준 설치본(kube-prometheus-stack, loki-stack, grafana chart) 기준 규칙.
var discoverRules = []svcRule{
	{
		key: "prometheus", label: "Prometheus",
		nameHints:   []string{"prometheus-operated", "prometheus-k8s", "prometheus-server", "prometheus"},
		portNames:   []string{"http-web", "web", "http"},
		portNumbers: []int32{9090},
		exclude:     []string{"alertmanager", "operator", "node-exporter", "kube-state-metrics", "pushgateway", "adapter"},
		installHint: "Prometheus가 없습니다. kube-prometheus-stack 설치를 진행하세요: " +
			"helm repo add prometheus-community https://prometheus-community.github.io/helm-charts && " +
			"helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack -n monitoring --create-namespace",
	},
	{
		key: "loki", label: "Loki",
		nameHints:   []string{"loki-gateway", "loki-query-frontend", "loki-read", "loki-stack", "loki"},
		portNames:   []string{"http-metrics", "http", "grpc"},
		portNumbers: []int32{3100, 80},
		exclude:     []string{"promtail", "canary", "memberlist", "headless", "alloy"},
		installHint: "Loki가 없습니다. 로그 수집이 필요하면 설치를 진행하세요: " +
			"helm repo add grafana https://grafana.github.io/helm-charts && " +
			"helm install loki grafana/loki-stack -n monitoring --create-namespace",
	},
	{
		key: "alertmanager", label: "Alertmanager",
		nameHints:   []string{"alertmanager-operated", "alertmanager-main", "alertmanager"},
		portNames:   []string{"http-web", "web", "http"},
		portNumbers: []int32{9093},
		exclude:     []string{"operator", "headless"},
		installHint: "Alertmanager가 없습니다. kube-prometheus-stack에 포함되어 있으니 함께 설치하세요. " +
			"설정하면 webhook 대신 폴링(pull) 방식으로 alert를 가져옵니다.",
	},
	{
		key: "grafana", label: "Grafana",
		nameHints:   []string{"grafana"},
		portNames:   []string{"http-web", "service", "http", "grafana"},
		portNumbers: []int32{3000, 80},
		exclude:     []string{"operator", "image-renderer", "headless", "agent"},
		installHint: "Grafana가 없습니다. 알림 딥링크용(선택 항목)이므로 없어도 동작합니다. " +
			"필요하면 kube-prometheus-stack에 포함된 Grafana를 사용하세요.",
	},
}

// handleCollectorDiscover는 현재 클러스터의 Service를 훑어 관측 스택 주소를 추정합니다.
// GET /api/collector/discover
//
// read-only(services list)만 사용하며, 찾지 못한 항목은 설치 안내와 함께 반환한다.
// in-cluster가 아니면(로컬 실행) available=false로 응답한다.
func (s *WebhookServer) handleCollectorDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	kube := NewKubeCollector()
	if kube == nil || kube.cs == nil {
		writeJSON(w, http.StatusOK, DiscoverResult{
			Available: false,
			Error: "in-cluster Kubernetes API에 연결할 수 없습니다. " +
				"자동조회는 클러스터 안에서 실행될 때만 동작합니다(로컬 docker compose 실행 시 미지원).",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	list, err := kube.cs.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		writeJSON(w, http.StatusOK, DiscoverResult{
			Available: false,
			Error:     fmt.Sprintf("Service 목록 조회 실패(RBAC 확인 필요): %v", err),
		})
		return
	}

	res := DiscoverResult{Available: true, Scanned: len(list.Items)}
	for _, rule := range discoverRules {
		if hit := matchService(rule, list.Items); hit.URL != "" {
			res.Found = append(res.Found, hit)
		} else {
			res.Missing = append(res.Missing, DiscoveredEndpoint{
				Key: rule.key, Label: rule.label, InstallHint: rule.installHint,
			})
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// matchService는 규칙에 맞는 Service를 찾아 in-cluster URL을 만든다.
// nameHints 순서가 우선순위이며, 같은 힌트 안에서는 포트 매칭이 정확한 것을 고른다.
func matchService(rule svcRule, svcs []corev1.Service) DiscoveredEndpoint {
	out := DiscoveredEndpoint{Key: rule.key, Label: rule.label}

	type scored struct {
		url     string
		svcDesc string
		rank    int // 낮을수록 우선
	}
	var hits []scored

	for _, svc := range svcs {
		// ExternalName은 in-cluster 주소를 만들 수 없어 제외한다.
		// headless(ClusterIP=None)는 제외하지 않는다 — kube-prometheus-stack의
		// prometheus-operated/alertmanager-operated가 headless이고, 이 프로젝트의
		// values.yaml·README가 관례적으로 그 주소를 쓴다(DNS가 pod IP로 해석됨).
		if svc.Spec.Type == corev1.ServiceTypeExternalName {
			continue
		}
		name := strings.ToLower(svc.Name)

		if hasAny(name, rule.exclude) {
			continue
		}
		hintRank := indexOfHint(name, rule.nameHints)
		if hintRank < 0 {
			continue
		}
		port, portRank := pickPort(svc, rule)
		if port == 0 {
			continue
		}
		hits = append(hits, scored{
			url:     fmt.Sprintf("http://%s.%s.svc:%d", svc.Name, svc.Namespace, port),
			svcDesc: fmt.Sprintf("%s/%s:%d", svc.Namespace, svc.Name, port),
			rank:    hintRank*10 + portRank,
		})
	}
	if len(hits) == 0 {
		return out
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		return hits[i].url < hits[j].url
	})

	out.URL = hits[0].url
	out.Service = hits[0].svcDesc
	for _, h := range hits[1:] {
		out.Candidates = append(out.Candidates, h.url)
	}
	return out
}

// indexOfHint는 name이 매칭되는 힌트의 인덱스를 반환한다(없으면 -1). 인덱스가 낮을수록 우선.
func indexOfHint(name string, hints []string) int {
	for i, h := range hints {
		if strings.Contains(name, h) {
			return i
		}
	}
	return -1
}

// pickPort는 규칙의 포트 번호/이름에 맞는 포트를 고른다. 번호 일치가 이름 일치보다 우선.
// 아무것도 맞지 않으면 0을 반환해 후보에서 제외한다(임의 포트 추측 금지).
func pickPort(svc corev1.Service, rule svcRule) (int32, int) {
	for _, p := range svc.Spec.Ports {
		for _, want := range rule.portNumbers {
			if p.Port == want {
				return p.Port, 0
			}
		}
	}
	for _, p := range svc.Spec.Ports {
		for _, want := range rule.portNames {
			if strings.EqualFold(p.Name, want) {
				return p.Port, 1
			}
		}
	}
	return 0, 0
}

func hasAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
