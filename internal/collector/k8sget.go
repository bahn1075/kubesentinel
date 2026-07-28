package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// gvrScope는 리소스 종류의 GroupVersionResource와 네임스페이스 스코프 여부다.
type gvrScope struct {
	gvr        schema.GroupVersionResource
	namespaced bool
}

func gvr(group, version, resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
}

// k8sAliases는 kind/단수/복수/축약형 → GVR+스코프 매핑이다(RBAC read-only 허용 목록과 일치).
// Secret은 의도적으로 제외한다(안전 가드). CRD 등 목록 외 종류는 확장 시 여기+RBAC에 추가.
var k8sAliases = map[string]gvrScope{
	// core/v1 (namespaced)
	"pod": {gvr("", "v1", "pods"), true}, "pods": {gvr("", "v1", "pods"), true}, "po": {gvr("", "v1", "pods"), true},
	"service": {gvr("", "v1", "services"), true}, "services": {gvr("", "v1", "services"), true}, "svc": {gvr("", "v1", "services"), true},
	"endpoints": {gvr("", "v1", "endpoints"), true}, "ep": {gvr("", "v1", "endpoints"), true},
	"configmap": {gvr("", "v1", "configmaps"), true}, "configmaps": {gvr("", "v1", "configmaps"), true}, "cm": {gvr("", "v1", "configmaps"), true},
	"serviceaccount": {gvr("", "v1", "serviceaccounts"), true}, "serviceaccounts": {gvr("", "v1", "serviceaccounts"), true}, "sa": {gvr("", "v1", "serviceaccounts"), true},
	"persistentvolumeclaim": {gvr("", "v1", "persistentvolumeclaims"), true}, "persistentvolumeclaims": {gvr("", "v1", "persistentvolumeclaims"), true}, "pvc": {gvr("", "v1", "persistentvolumeclaims"), true},
	"event": {gvr("", "v1", "events"), true}, "events": {gvr("", "v1", "events"), true},
	"replicationcontroller": {gvr("", "v1", "replicationcontrollers"), true}, "rc": {gvr("", "v1", "replicationcontrollers"), true},
	"resourcequota": {gvr("", "v1", "resourcequotas"), true}, "resourcequotas": {gvr("", "v1", "resourcequotas"), true}, "quota": {gvr("", "v1", "resourcequotas"), true},
	"limitrange": {gvr("", "v1", "limitranges"), true}, "limitranges": {gvr("", "v1", "limitranges"), true},
	// core/v1 (cluster)
	"node": {gvr("", "v1", "nodes"), false}, "nodes": {gvr("", "v1", "nodes"), false}, "no": {gvr("", "v1", "nodes"), false},
	"namespace": {gvr("", "v1", "namespaces"), false}, "namespaces": {gvr("", "v1", "namespaces"), false}, "ns": {gvr("", "v1", "namespaces"), false},
	"persistentvolume": {gvr("", "v1", "persistentvolumes"), false}, "persistentvolumes": {gvr("", "v1", "persistentvolumes"), false}, "pv": {gvr("", "v1", "persistentvolumes"), false},
	// apps/v1 (namespaced)
	"deployment": {gvr("apps", "v1", "deployments"), true}, "deployments": {gvr("apps", "v1", "deployments"), true}, "deploy": {gvr("apps", "v1", "deployments"), true},
	"replicaset": {gvr("apps", "v1", "replicasets"), true}, "replicasets": {gvr("apps", "v1", "replicasets"), true}, "rs": {gvr("apps", "v1", "replicasets"), true},
	"statefulset": {gvr("apps", "v1", "statefulsets"), true}, "statefulsets": {gvr("apps", "v1", "statefulsets"), true}, "sts": {gvr("apps", "v1", "statefulsets"), true},
	"daemonset": {gvr("apps", "v1", "daemonsets"), true}, "daemonsets": {gvr("apps", "v1", "daemonsets"), true}, "ds": {gvr("apps", "v1", "daemonsets"), true},
	// batch/v1 (namespaced)
	"job": {gvr("batch", "v1", "jobs"), true}, "jobs": {gvr("batch", "v1", "jobs"), true},
	"cronjob": {gvr("batch", "v1", "cronjobs"), true}, "cronjobs": {gvr("batch", "v1", "cronjobs"), true}, "cj": {gvr("batch", "v1", "cronjobs"), true},
	// networking.k8s.io/v1
	"ingress": {gvr("networking.k8s.io", "v1", "ingresses"), true}, "ingresses": {gvr("networking.k8s.io", "v1", "ingresses"), true}, "ing": {gvr("networking.k8s.io", "v1", "ingresses"), true},
	"networkpolicy": {gvr("networking.k8s.io", "v1", "networkpolicies"), true}, "networkpolicies": {gvr("networking.k8s.io", "v1", "networkpolicies"), true}, "netpol": {gvr("networking.k8s.io", "v1", "networkpolicies"), true},
	"ingressclass": {gvr("networking.k8s.io", "v1", "ingressclasses"), false}, "ingressclasses": {gvr("networking.k8s.io", "v1", "ingressclasses"), false},
	// autoscaling/v2
	"horizontalpodautoscaler": {gvr("autoscaling", "v2", "horizontalpodautoscalers"), true}, "horizontalpodautoscalers": {gvr("autoscaling", "v2", "horizontalpodautoscalers"), true}, "hpa": {gvr("autoscaling", "v2", "horizontalpodautoscalers"), true},
	// policy/v1
	"poddisruptionbudget": {gvr("policy", "v1", "poddisruptionbudgets"), true}, "poddisruptionbudgets": {gvr("policy", "v1", "poddisruptionbudgets"), true}, "pdb": {gvr("policy", "v1", "poddisruptionbudgets"), true},
	// storage.k8s.io/v1
	"storageclass": {gvr("storage.k8s.io", "v1", "storageclasses"), false}, "storageclasses": {gvr("storage.k8s.io", "v1", "storageclasses"), false}, "sc": {gvr("storage.k8s.io", "v1", "storageclasses"), false},
	// discovery.k8s.io/v1
	"endpointslice": {gvr("discovery.k8s.io", "v1", "endpointslices"), true}, "endpointslices": {gvr("discovery.k8s.io", "v1", "endpointslices"), true},
}

const k8sGetMaxChars = 6000

// GetResource는 임의 종류의 리소스를 조회한다(kubectl get -o json 상당). 안전 가드 내장:
// Secret 조회 거부, 네임스페이스 리소스는 namespace 필수(전체-네임스페이스 덤프 방지), 출력 절삭.
func (k *KubeCollector) GetResource(kind, namespace, name string) string {
	if k == nil || k.dyn == nil {
		return "ERROR: kubernetes dynamic client not available (not in-cluster)"
	}
	key := strings.ToLower(strings.TrimSpace(kind))
	if key == "secret" || key == "secrets" {
		return "ERROR: reading Secrets is not permitted (safety guard). Diagnose from non-secret resources."
	}
	gs, ok := k8sAliases[key]
	if !ok {
		return fmt.Sprintf("ERROR: unsupported kind %q. supported: %s", kind, supportedKinds())
	}
	if gs.namespaced && namespace == "" {
		return fmt.Sprintf("ERROR: namespace is required for namespaced kind %q (cluster-wide listing is disabled)", kind)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	ri := k.dyn.Resource(gs.gvr)
	if gs.namespaced {
		if name == "" {
			list, err := ri.Namespace(namespace).List(ctx, metav1.ListOptions{Limit: 50})
			if err != nil {
				return "ERROR: " + err.Error()
			}
			return listSummary(kind, list.Items)
		}
		obj, err := ri.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return objJSON(obj.Object)
	}
	// cluster-scoped
	if name == "" {
		list, err := ri.List(ctx, metav1.ListOptions{Limit: 50})
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return listSummary(kind, list.Items)
	}
	obj, err := ri.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "ERROR: " + err.Error()
	}
	return objJSON(obj.Object)
}

// GetLogs는 파드 컨테이너 로그를 조회한다(kubectl logs 상당). previous=true면 직전 종료 컨테이너 로그.
func (k *KubeCollector) GetLogs(namespace, pod, container string, previous bool, tail int) string {
	if k == nil || k.cs == nil {
		return "ERROR: kubernetes API not available (not in-cluster)"
	}
	if namespace == "" || pod == "" {
		return "ERROR: 'namespace' and 'pod' are required"
	}
	if tail <= 0 || tail > 300 {
		tail = 100
	}
	t := int64(tail)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := &corev1.PodLogOptions{Container: container, Previous: previous, TailLines: &t}
	stream, err := k.cs.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	if err != nil {
		return "ERROR: " + err.Error()
	}
	defer stream.Close()
	b, _ := io.ReadAll(io.LimitReader(stream, k8sGetMaxChars+1024))
	out := string(b)
	if out == "" {
		return "(no log output)"
	}
	return truncateStr(out, k8sGetMaxChars)
}

// objJSON은 unstructured 오브젝트에서 노이즈(managedFields/last-applied)를 제거하고 JSON으로 절삭 반환.
func objJSON(obj map[string]interface{}) string {
	trimObject(obj)
	b, err := json.MarshalIndent(obj, "", " ")
	if err != nil {
		return "ERROR: marshal: " + err.Error()
	}
	return truncateStr(string(b), k8sGetMaxChars)
}

// listSummary는 목록 조회 시 이름/상태 위주 요약을 반환한다(전체 오브젝트 덤프 방지).
func listSummary(kind string, items []unstructured.Unstructured) string {
	if len(items) == 0 {
		return "(no " + kind + ")"
	}
	var lines []string
	for i := range items {
		obj := items[i].Object
		name, _, _ := unstructured.NestedString(obj, "metadata", "name")
		ns, _, _ := unstructured.NestedString(obj, "metadata", "namespace")
		phase, ok, _ := unstructured.NestedString(obj, "status", "phase")
		line := name
		if ns != "" {
			line = ns + "/" + name
		}
		if ok && phase != "" {
			line += " [" + phase + "]"
		}
		lines = append(lines, line)
	}
	sort.Strings(lines)
	head := fmt.Sprintf("%d %s (name only; call with a specific name for full object):\n", len(lines), kind)
	return truncateStr(head+strings.Join(lines, "\n"), k8sGetMaxChars)
}

// trimObject는 오브젝트에서 부피 큰/무의미한 필드를 제거한다.
func trimObject(obj map[string]interface{}) {
	if md, ok := obj["metadata"].(map[string]interface{}); ok {
		delete(md, "managedFields")
		if ann, ok := md["annotations"].(map[string]interface{}); ok {
			delete(ann, "kubectl.kubernetes.io/last-applied-configuration")
			if len(ann) == 0 {
				delete(md, "annotations")
			}
		}
	}
}

func supportedKinds() string {
	set := map[string]bool{}
	for _, gs := range k8sAliases {
		set[gs.gvr.Resource] = true
	}
	out := make([]string, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncated)"
}
