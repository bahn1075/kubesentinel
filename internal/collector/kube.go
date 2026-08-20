package collector

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"kubesentinel-ai/internal/models"
)

// KubeCollector는 in-cluster Kubernetes API에서 Events·리소스 상태를 수집합니다. (L2 근거 보강)
// in-cluster 설정이 없으면(로컬 실행 등) nil이 되어 자동 skip 된다(best-effort).
type KubeCollector struct {
	cs        *kubernetes.Clientset
	dyn       dynamic.Interface // 범용 리소스 조회(k8s_get)용
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
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("[KubeSentinel] dynamic client init failed (k8s_get disabled): %v\n", err)
	}
	return &KubeCollector{cs: cs, dyn: dyn, maxEvents: 20}
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

// Events는 agentic 도구용 공개 래퍼: 네임스페이스(+선택 객체)의 최근 이벤트.
func (k *KubeCollector) Events(ns, name string) []string {
	if k == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return k.events(ctx, ns, name)
}

// ListPods는 네임스페이스의 pod와 상태(phase, waiting reason, restarts)를 요약한다(agentic 도구용).
func (k *KubeCollector) ListPods(ns string) []string {
	if k == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	pods, err := k.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{Limit: 100})
	if err != nil {
		return []string{"ERROR: " + err.Error()}
	}
	out := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		reason := ""
		restarts := int32(0)
		for _, cs := range p.Status.ContainerStatuses {
			restarts += cs.RestartCount
			if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
				reason = cs.State.Waiting.Reason
			} else if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
				reason = cs.State.Terminated.Reason
			}
		}
		line := fmt.Sprintf("%s: %s", p.Name, p.Status.Phase)
		if reason != "" {
			line += " (" + reason + ")"
		}
		if restarts > 0 {
			line += fmt.Sprintf(" restarts=%d", restarts)
		}
		out = append(out, line)
	}
	return out
}

// RestartDeployment는 지정된 Deployment에 rollout-restart 주석을 패치해 재시작을 트리거한다.
// self-restart(AI 설정 반영) 전용 — RBAC은 이 Deployment 1개로 scope된 patch 권한만 부여된다(rbac-restart.yaml).
func (k *KubeCollector) RestartDeployment(ns, name string) error {
	if k == nil {
		return fmt.Errorf("kubernetes client를 사용할 수 없습니다 (in-cluster 아님)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`,
		time.Now().Format(time.RFC3339))
	_, err := k.cs.AppsV1().Deployments(ns).Patch(ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	return err
}

// NodeArchs는 클러스터 노드의 고유 아키텍처 집합을 반환한다(예: ["arm64"]). (프로브/도구용)
func (k *KubeCollector) NodeArchs() []string {
	if k == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	nodes, err := k.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, n := range nodes.Items {
		if a := n.Status.NodeInfo.Architecture; a != "" {
			set[a] = true
		}
	}
	archs := make([]string, 0, len(set))
	for a := range set {
		archs = append(archs, a)
	}
	sort.Strings(archs)
	return archs
}

// NodeInfoSummary는 노드 arch·수·비정상 여부를 사람이 읽는 요약으로 반환한다(agentic 도구용).
func (k *KubeCollector) NodeInfoSummary() []string {
	if k == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	nodes, err := k.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{"ERROR: " + err.Error()}
	}
	archCount := map[string]int{}
	out := make([]string, 0, len(nodes.Items)+1)
	for _, n := range nodes.Items {
		arch := n.Status.NodeInfo.Architecture
		archCount[arch]++
		ready := "NotReady"
		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" && c.Status == "True" {
				ready = "Ready"
			}
		}
		taints := ""
		if len(n.Spec.Taints) > 0 {
			ts := make([]string, 0, len(n.Spec.Taints))
			for _, t := range n.Spec.Taints {
				ts = append(ts, t.Key+"="+string(t.Effect))
			}
			taints = " taints=[" + strings.Join(ts, ",") + "]"
		}
		out = append(out, fmt.Sprintf("%s: arch=%s %s%s", n.Name, arch, ready, taints))
	}
	summary := make([]string, 0, len(archCount))
	for a, c := range archCount {
		summary = append(summary, fmt.Sprintf("%s×%d", a, c))
	}
	sort.Strings(summary)
	out = append([]string{"node architectures: " + strings.Join(summary, ", ")}, out...)
	return out
}

// ContainerDiag는 컨테이너의 진단 신호(종료코드/사유/재시작/리소스)를 담는다. (프로브용)
type ContainerDiag struct {
	Name                   string
	Image                  string
	RestartCount           int32
	WaitingReason          string
	TerminatedReason       string
	TerminatedExitCode     int32
	LastTerminatedReason   string
	LastTerminatedExitCode int32
	MemLimit               string
	MemRequest             string
	CPURequest             string
}

// PodContainers는 파드 컨테이너의 상태·리소스 신호를 반환한다(프로브용). 없으면 nil.
func (k *KubeCollector) PodContainers(ns, pod string) []ContainerDiag {
	if k == nil || ns == "" || pod == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	p, err := k.cs.CoreV1().Pods(ns).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	// spec의 리소스 요청/제한을 컨테이너명으로 인덱싱
	reqLim := map[string][3]string{} // name → {memLimit, memReq, cpuReq}
	for _, c := range append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...) {
		ml := c.Resources.Limits.Memory().String()
		mr := c.Resources.Requests.Memory().String()
		cr := c.Resources.Requests.Cpu().String()
		reqLim[c.Name] = [3]string{ml, mr, cr}
	}
	var out []ContainerDiag
	statuses := append(append([]corev1.ContainerStatus{}, p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...)
	for _, cs := range statuses {
		d := ContainerDiag{Name: cs.Name, Image: cs.Image, RestartCount: cs.RestartCount}
		if cs.State.Waiting != nil {
			d.WaitingReason = cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil {
			d.TerminatedReason = cs.State.Terminated.Reason
			d.TerminatedExitCode = cs.State.Terminated.ExitCode
		}
		if cs.LastTerminationState.Terminated != nil {
			d.LastTerminatedReason = cs.LastTerminationState.Terminated.Reason
			d.LastTerminatedExitCode = cs.LastTerminationState.Terminated.ExitCode
		}
		if rl, ok := reqLim[cs.Name]; ok {
			d.MemLimit, d.MemRequest, d.CPURequest = rl[0], rl[1], rl[2]
		}
		out = append(out, d)
	}
	return out
}

// PodImages는 파드(및 대체로 잡의 첫 파드) 컨테이너 이미지 목록을 반환한다(init 포함). (프로브/도구용)
func (k *KubeCollector) PodImages(ns, pod string) []string {
	if k == nil || ns == "" || pod == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	p, err := k.cs.CoreV1().Pods(ns).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...) {
		if c.Image != "" && !seen[c.Image] {
			seen[c.Image] = true
			out = append(out, c.Image)
		}
	}
	return out
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
