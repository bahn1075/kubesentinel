package collector

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"kubesentinel-ai/internal/models"
)

// KubeCollector는 in-cluster Kubernetes API에서 Events·리소스 상태를 수집합니다. (L2 근거 보강)
// in-cluster 설정이 없으면(로컬 실행 등) nil이 되어 자동 skip 된다(best-effort).
type KubeCollector struct {
	cs        *kubernetes.Clientset
	maxEvents int
}

// NewKubeCollector는 in-cluster config로 clientset을 만든다. 불가하면 nil.
func NewKubeCollector() *KubeCollector {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("[KubeSentinel] KubeCollector disabled (not in-cluster): %v\n", err)
		return nil
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("[KubeSentinel] KubeCollector init failed: %v\n", err)
		return nil
	}
	return &KubeCollector{cs: cs, maxEvents: 20}
}

// Enrich는 bundle에 Kubernetes Events·리소스 상태·노드 상태를 in-place로 보강한다(best-effort).
func (k *KubeCollector) Enrich(b *models.EvidenceBundle) {
	if k == nil || b == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if b.Namespace != "" {
		// 대상 네임스페이스의 최근 이벤트 (pod 있으면 해당 객체로 좁힘, 없으면 네임스페이스 전체)
		obj := b.Pod
		b.Events = append(b.Events, k.events(ctx, b.Namespace, obj)...)
		if st := k.resourceStatus(ctx, b); len(st) > 0 {
			b.ResourceYAML = st
		}
	} else {
		// 네임스페이스 없는 인프라/컨트롤플레인 alert → 노드 상태 + kube-system 경고 이벤트
		b.Events = append(b.Events, k.nodeHealth(ctx)...)
		b.Events = append(b.Events, k.events(ctx, "kube-system", "")...)
	}
}

// events는 네임스페이스의 최근 이벤트를 최신순으로 문자열 목록으로 반환한다.
// objName이 있으면 involvedObject.name으로 좁힌다.
func (k *KubeCollector) events(ctx context.Context, ns, objName string) []string {
	opts := metav1.ListOptions{Limit: 200}
	if objName != "" {
		opts.FieldSelector = "involvedObject.name=" + objName
	}
	list, err := k.cs.CoreV1().Events(ns).List(ctx, opts)
	if err != nil || len(list.Items) == 0 {
		return nil
	}
	items := list.Items
	sort.Slice(items, func(i, j int) bool {
		return evTime(items[i]).After(evTime(items[j]))
	})
	if len(items) > k.maxEvents {
		items = items[:k.maxEvents]
	}
	out := make([]string, 0, len(items))
	for _, e := range items {
		cnt := ""
		if e.Count > 1 {
			cnt = fmt.Sprintf(" (x%d)", e.Count)
		}
		out = append(out, fmt.Sprintf("%s %s [%s/%s] %s%s",
			e.Type, e.Reason, e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Message, cnt))
	}
	return out
}

// resourceStatus는 alert 대상 리소스의 현재 상태 요약을 반환한다(종류별 분기).
func (k *KubeCollector) resourceStatus(ctx context.Context, b *models.EvidenceBundle) map[string]interface{} {
	ns, name := b.Namespace, b.Workload
	switch b.Kind {
	case "Pod":
		p, err := k.cs.CoreV1().Pods(ns).Get(ctx, b.Pod, metav1.GetOptions{})
		if err != nil {
			return nil
		}
		st := map[string]interface{}{"kind": "Pod", "phase": string(p.Status.Phase)}
		waits := []string{}
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Waiting != nil {
				waits = append(waits, fmt.Sprintf("%s: %s", cs.Name, cs.State.Waiting.Reason))
			}
			if cs.RestartCount > 0 {
				waits = append(waits, fmt.Sprintf("%s restarts=%d", cs.Name, cs.RestartCount))
			}
		}
		if len(waits) > 0 {
			st["containers"] = waits
		}
		return st
	case "Deployment":
		d, err := k.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil
		}
		return map[string]interface{}{"kind": "Deployment", "replicas": d.Status.Replicas,
			"ready": d.Status.ReadyReplicas, "available": d.Status.AvailableReplicas, "unavailable": d.Status.UnavailableReplicas}
	case "StatefulSet":
		s, err := k.cs.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil
		}
		return map[string]interface{}{"kind": "StatefulSet", "replicas": s.Status.Replicas, "ready": s.Status.ReadyReplicas}
	case "DaemonSet":
		ds, err := k.cs.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil
		}
		return map[string]interface{}{"kind": "DaemonSet", "desired": ds.Status.DesiredNumberScheduled,
			"ready": ds.Status.NumberReady, "unavailable": ds.Status.NumberUnavailable}
	case "Job":
		j, err := k.cs.BatchV1().Jobs(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil
		}
		st := map[string]interface{}{"kind": "Job", "active": j.Status.Active, "succeeded": j.Status.Succeeded, "failed": j.Status.Failed}
		for _, c := range j.Status.Conditions {
			if c.Status == "True" {
				st["condition"] = fmt.Sprintf("%s: %s", c.Type, c.Message)
			}
		}
		return st
	}
	return nil
}

// nodeHealth는 Ready가 아니거나 압박(pressure) 상태인 노드를 요약한다.
func (k *KubeCollector) nodeHealth(ctx context.Context) []string {
	nodes, err := k.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	out := []string{}
	for _, n := range nodes.Items {
		for _, c := range n.Status.Conditions {
			bad := (c.Type == "Ready" && c.Status != "True") ||
				(c.Type != "Ready" && c.Status == "True") // *Pressure/NetworkUnavailable = True는 이상
			if bad {
				out = append(out, fmt.Sprintf("Node %s: %s=%s (%s)", n.Name, c.Type, c.Status, c.Reason))
			}
		}
	}
	if len(out) == 0 {
		out = append(out, fmt.Sprintf("All %d nodes Ready (no node-level condition anomalies)", len(nodes.Items)))
	}
	return out
}

// evTime는 이벤트의 최신 시각(구형 LastTimestamp 우선, 없으면 신형 EventTime)을 반환한다.
func evTime(e corev1.Event) time.Time {
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	return e.EventTime.Time
}
