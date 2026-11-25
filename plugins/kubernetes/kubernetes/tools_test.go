package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

// newCluster is a fake clientset that behaves like the API server where the
// fake on its own does not: every object has a resourceVersion and a real
// patch bumps it, and a dry-run patch is applied to a copy with nothing
// stored. The resourceVersion test inside every patch is enforced by the
// JSON patch itself, as on a real server.
func newCluster(t *testing.T, objects ...runtime.Object) *fake.Clientset {
	t.Helper()
	for _, o := range objects {
		if m, ok := o.(metav1.Object); ok && m.GetResourceVersion() == "" {
			m.SetResourceVersion("1")
		}
	}
	c := fake.NewClientset(objects...)
	c.PrependReactor("patch", "*", func(a clienttesting.Action) (bool, runtime.Object, error) {
		pa := a.(clienttesting.PatchActionImpl)
		obj, err := c.Tracker().Get(pa.GetResource(), pa.GetNamespace(), pa.GetName())
		if err != nil {
			return true, nil, err
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return true, nil, err
		}
		patch, err := jsonpatch.DecodePatch(pa.GetPatch())
		if err != nil {
			return true, nil, err
		}
		out, err := patch.Apply(data)
		if err != nil {
			return true, nil, err
		}
		fresh := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
		if err := json.Unmarshal(out, fresh); err != nil {
			return true, nil, err
		}
		if len(pa.GetPatchOptions().DryRun) > 0 {
			return true, fresh, nil
		}
		m := fresh.(metav1.Object)
		rv, _ := strconv.Atoi(m.GetResourceVersion())
		m.SetResourceVersion(strconv.Itoa(rv + 1))
		return true, fresh, c.Tracker().Update(pa.GetResource(), fresh, pa.GetNamespace())
	})
	return c
}

func engine(t *testing.T, c *fake.Clientset, config ToolsConfig) *ToolEngine {
	t.Helper()
	e, err := NewToolEngine(c, config)
	require.NoError(t, err)
	e.now = func() time.Time { return time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC) }
	return e
}

func request(tool string, params map[string]any) pluginsdk.ToolRequest {
	return pluginsdk.ToolRequest{Tool: tool, Params: params, Identity: "user:alice", Origin: "task_run:42"}
}

func deployment(ns, name string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, UID: types.UID(name + "-uid"), Labels: map[string]string{"app": name}},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app:v2"}}},
			},
		},
	}
}

// planAndExecute plans, checks the plan wrote nothing, then executes with
// the plan's precondition.
func planAndExecute(t *testing.T, c *fake.Clientset, e *ToolEngine, req pluginsdk.ToolRequest) (*pluginsdk.ToolPlan, *pluginsdk.ToolResult) {
	t.Helper()
	ctx := context.Background()
	writes := countWrites(c)
	plan, err := e.Plan(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, writes, countWrites(c), "a plan must persist nothing")
	require.NotEmpty(t, plan.Precondition)

	req.Precondition = plan.Precondition
	result, err := e.Execute(ctx, req)
	require.NoError(t, err)
	return plan, result
}

func countWrites(c *fake.Clientset) int {
	n := 0
	for _, a := range c.Actions() {
		if pa, ok := a.(clienttesting.PatchActionImpl); ok && len(pa.GetPatchOptions().DryRun) == 0 {
			n++
		}
	}
	return n
}

func TestToolManifestIsValid(t *testing.T) {
	require.NoError(t, pluginsdk.ValidateManifest(ToolSpecs()))
}

func TestScale(t *testing.T) {
	c := newCluster(t, deployment("payments", "api", 3))
	e := engine(t, c, ToolsConfig{})
	req := request(toolScale, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": int64(5)})

	plan, result := planAndExecute(t, c, e, req)
	assert.Equal(t, "deployment/payments/api", plan.Target)
	assert.Equal(t, kindDeployment, plan.TargetKind)
	assert.JSONEq(t, `{"spec":{"replicas":3}}`, string(plan.Before))
	assert.JSONEq(t, `{"metadata":{"annotations":{"marmot.io/actor":"user:alice","marmot.io/origin":"task_run:42"}},"spec":{"replicas":5}}`, string(plan.After))
	assert.Contains(t, plan.Diff, "- spec.replicas: 3\n+ spec.replicas: 5\n")

	dep, err := c.AppsV1().Deployments("payments").Get(context.Background(), "api", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(5), *dep.Spec.Replicas)
	assert.Equal(t, "task_run:42", dep.Annotations[annotationOrigin])
	assert.Equal(t, "user:alice", dep.Annotations[annotationActor])

	require.NotNil(t, result.Undo)
	assert.Equal(t, toolScale, result.Undo.Tool)
	assert.Equal(t, int64(3), result.Undo.Params["replicas"])

	for _, a := range c.Actions() {
		if pa, ok := a.(clienttesting.PatchActionImpl); ok {
			assert.Equal(t, fieldManager, pa.GetPatchOptions().FieldManager)
			assert.Equal(t, types.JSONPatchType, pa.GetPatchType())
		}
	}
}

func TestScaleWarnings(t *testing.T) {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Namespace: "payments", Name: "api"},
		Spec:       autoscalingv2.HorizontalPodAutoscalerSpec{ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "api"}},
	}
	c := newCluster(t, deployment("payments", "api", 3), hpa)
	plan, err := engine(t, c, ToolsConfig{}).Plan(context.Background(),
		request(toolScale, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": int64(0)}))
	require.NoError(t, err)
	assert.Len(t, plan.Warnings, 2, "zero replicas and the autoscaler")
}

func TestScaleRespectsMaxReplicas(t *testing.T) {
	c := newCluster(t, deployment("payments", "api", 3))
	req := request(toolScale, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": int64(21)})

	_, err := engine(t, c, ToolsConfig{}).Plan(context.Background(), req)
	require.ErrorContains(t, err, "max_replicas of 20")

	_, err = engine(t, c, ToolsConfig{MaxReplicas: 50}).Plan(context.Background(), req)
	require.NoError(t, err)
}

func TestExecuteRefusesStaleState(t *testing.T) {
	ctx := context.Background()
	c := newCluster(t, deployment("payments", "api", 3))
	e := engine(t, c, ToolsConfig{})
	req := request(toolScale, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": int64(5)})

	plan, err := e.Plan(ctx, req)
	require.NoError(t, err)

	// Someone else changes the Deployment between the plan and its execution.
	dep, err := c.AppsV1().Deployments("payments").Get(ctx, "api", metav1.GetOptions{})
	require.NoError(t, err)
	dep.Spec.Replicas = ptr.To(int32(4))
	dep.ResourceVersion = "2"
	_, err = c.AppsV1().Deployments("payments").Update(ctx, dep, metav1.UpdateOptions{})
	require.NoError(t, err)

	req.Precondition = plan.Precondition
	_, err = e.Execute(ctx, req)
	require.ErrorIs(t, err, ErrStale)

	req.Precondition = ""
	_, err = e.Execute(ctx, req)
	require.Error(t, err, "no precondition, no execution")
}

// The resourceVersion test travels in the patch itself, so a change that
// lands after the engine's own check is still refused.
func TestPatchCarriesThePrecondition(t *testing.T) {
	dep := deployment("payments", "api", 3)
	dep.ResourceVersion = "7"
	e := engine(t, newCluster(t), ToolsConfig{})
	data := e.patchFor(dep, "7", &change{}, request(toolScale, nil))

	stale, err := json.Marshal(func() *appsv1.Deployment { d := dep.DeepCopy(); d.ResourceVersion = "8"; return d }())
	require.NoError(t, err)
	patch, err := jsonpatch.DecodePatch(data)
	require.NoError(t, err)
	_, err = patch.Apply(stale)
	require.Error(t, err)
}

func TestGitOpsManagedObjectsAreRefused(t *testing.T) {
	cases := map[string]func(*appsv1.Deployment){
		"flux kustomization": func(d *appsv1.Deployment) { d.Labels["kustomize.toolkit.fluxcd.io/name"] = "apps" },
		"flux helm release":  func(d *appsv1.Deployment) { d.Labels["helm.toolkit.fluxcd.io/name"] = "api" },
		"argo cd label":      func(d *appsv1.Deployment) { d.Labels["argocd.argoproj.io/instance"] = "api" },
		"argo cd tracking": func(d *appsv1.Deployment) {
			d.Annotations = map[string]string{"argocd.argoproj.io/tracking-id": "api:apps/Deployment:payments/api"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			dep := deployment("payments", "api", 3)
			mutate(dep)
			c := newCluster(t, dep)
			req := request(toolScale, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": int64(5)})

			_, err := engine(t, c, ToolsConfig{}).Plan(context.Background(), req)
			require.ErrorIs(t, err, ErrGitOpsManaged)
			assert.Contains(t, err.Error(), "Git")

			_, err = engine(t, c, ToolsConfig{AllowGitOpsManaged: true}).Plan(context.Background(), req)
			require.NoError(t, err)
		})
	}

	// A Helm chart's instance label alone is not GitOps.
	dep := deployment("payments", "api", 3)
	dep.Labels["app.kubernetes.io/instance"] = "api"
	_, err := engine(t, newCluster(t, dep), ToolsConfig{}).Plan(context.Background(),
		request(toolScale, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": int64(5)}))
	require.NoError(t, err)
}

func TestScope(t *testing.T) {
	ctx := context.Background()
	c := newCluster(t, deployment("payments", "api", 3), deployment("kube-system", "coredns", 2))
	scale := func(ns, name string) pluginsdk.ToolRequest {
		return request(toolScale, map[string]any{"kind": "deployment", "namespace": ns, "name": name, "replicas": int64(1)})
	}

	before := len(c.Actions())
	_, err := engine(t, c, ToolsConfig{}).Plan(ctx, scale("kube-system", "coredns"))
	require.ErrorIs(t, err, ErrOutOfScope)
	assert.Len(t, c.Actions(), before, "an out of scope namespace is not even read")

	_, err = engine(t, c, ToolsConfig{Namespaces: []string{"kube-system"}}).Plan(ctx, scale("kube-system", "coredns"))
	require.NoError(t, err, "naming a system namespace allows it")

	_, err = engine(t, c, ToolsConfig{Namespaces: []string{"orders"}}).Plan(ctx, scale("payments", "api"))
	require.ErrorIs(t, err, ErrOutOfScope)

	_, err = engine(t, c, ToolsConfig{Selector: "marmot.io/tools=enabled"}).Plan(ctx, scale("payments", "api"))
	require.ErrorIs(t, err, ErrOutOfScope)

	_, err = engine(t, c, ToolsConfig{Selector: "app=api"}).Plan(ctx, scale("payments", "api"))
	require.NoError(t, err)

	_, err = NewToolEngine(c, ToolsConfig{Selector: "=bad"})
	require.Error(t, err)
}

func TestRolloutRestart(t *testing.T) {
	c := newCluster(t, deployment("payments", "api", 3))
	plan, result := planAndExecute(t, c, engine(t, c, ToolsConfig{}),
		request(toolRolloutRestart, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api"}))
	assert.Contains(t, plan.Diff, `spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]: "2026-11-25T09:00:00Z"`)
	assert.Nil(t, result.Undo, "restart is self healing, not reversible")

	dep, err := c.AppsV1().Deployments("payments").Get(context.Background(), "api", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "2026-11-25T09:00:00Z", dep.Spec.Template.Annotations[restartedAtAnnotation])
	assert.Equal(t, "app:v2", dep.Spec.Template.Spec.Containers[0].Image, "nothing else in the template changes")

	paused := deployment("payments", "paused", 1)
	paused.Spec.Paused = true
	_, err = engine(t, newCluster(t, paused), ToolsConfig{}).Plan(context.Background(),
		request(toolRolloutRestart, map[string]any{"kind": "deployment", "namespace": "payments", "name": "paused"}))
	require.ErrorContains(t, err, "paused")
}

func TestRolloutRestartDaemonSet(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "payments", Name: "agent"},
		Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"keep": "me"}},
		}},
	}
	c := newCluster(t, ds)
	planAndExecute(t, c, engine(t, c, ToolsConfig{}),
		request(toolRolloutRestart, map[string]any{"kind": "daemonset", "namespace": "payments", "name": "agent"}))

	got, err := c.AppsV1().DaemonSets("payments").Get(context.Background(), "agent", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "me", got.Spec.Template.Annotations["keep"], "existing template annotations survive")
	assert.NotEmpty(t, got.Spec.Template.Annotations[restartedAtAnnotation])
}

func replicaSet(dep *appsv1.Deployment, revision, image string) *appsv1.ReplicaSet {
	template := dep.Spec.Template.DeepCopy()
	template.Labels = map[string]string{"app": dep.Name, appsv1.DefaultDeploymentUniqueLabelKey: "hash-" + revision}
	template.Spec.Containers[0].Image = image
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dep.Namespace, Name: dep.Name + "-" + revision, Labels: template.Labels,
			Annotations:     map[string]string{revisionAnnotation: revision},
			OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: dep.Name, UID: dep.UID, Controller: ptr.To(true)}},
		},
		Spec: appsv1.ReplicaSetSpec{Template: *template},
	}
}

func TestRolloutUndo(t *testing.T) {
	dep := deployment("payments", "api", 3)
	dep.Annotations = map[string]string{revisionAnnotation: "2"}
	stranger := replicaSet(deployment("payments", "other", 1), "1", "evil:latest")
	stranger.Labels = map[string]string{"app": "api"} // matches the selector, but another owner
	c := newCluster(t, dep, replicaSet(dep, "1", "app:v1"), replicaSet(dep, "2", "app:v2"), stranger)

	plan, result := planAndExecute(t, c, engine(t, c, ToolsConfig{}),
		request(toolRolloutUndo, map[string]any{"namespace": "payments", "name": "api"}))
	assert.Contains(t, plan.Summary, "from revision 2 to revision 1")

	got, err := c.AppsV1().Deployments("payments").Get(context.Background(), "api", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "app:v1", got.Spec.Template.Spec.Containers[0].Image)
	assert.NotContains(t, got.Spec.Template.Labels, appsv1.DefaultDeploymentUniqueLabelKey)

	require.NotNil(t, result.Undo)
	assert.Equal(t, toolRolloutUndo, result.Undo.Tool)
	assert.Equal(t, int64(2), result.Undo.Params["to_revision"])

	_, err = engine(t, c, ToolsConfig{}).Plan(context.Background(),
		request(toolRolloutUndo, map[string]any{"namespace": "payments", "name": "api", "to_revision": int64(9)}))
	require.ErrorContains(t, err, "no revision 9")
}

func node(name string, unschedulable bool) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Unschedulable: unschedulable},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
	}
}

// A tainted control plane and a node that is not Ready are nowhere a workload can go, so neither keeps the last worker cordonable. Found against a real kind cluster, where the control plane counted.
func TestCordonCountsOnlyNodesThatTakePods(t *testing.T) {
	controlPlane := node("cp", false)
	controlPlane.Spec.Taints = []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}
	notReady := node("down", false)
	notReady.Status.Conditions[0].Status = corev1.ConditionFalse
	c := newCluster(t, controlPlane, notReady, node("worker", false))
	e := engine(t, c, ToolsConfig{})
	_, err := e.Plan(t.Context(), request(toolCordon, map[string]any{"name": "worker"}))
	require.ErrorIs(t, err, ErrLastNode)
	assert.Equal(t, codes.FailedPrecondition, status.Code(wireError(err)))
}

func TestWireErrorCodes(t *testing.T) {
	assert.Equal(t, codes.FailedPrecondition, status.Code(wireError(fmt.Errorf("x: %w", ErrStale))))
	assert.Equal(t, codes.PermissionDenied, status.Code(wireError(fmt.Errorf("x: %w", ErrOutOfScope))))
	assert.Equal(t, codes.PermissionDenied, status.Code(wireError(fmt.Errorf("x: %w", ErrGitOpsManaged))))
	assert.Nil(t, wireError(nil))
}

func TestCordon(t *testing.T) {
	c := newCluster(t, node("a", false), node("b", false))
	e := engine(t, c, ToolsConfig{})
	_, result := planAndExecute(t, c, e, request(toolCordon, map[string]any{"name": "a"}))
	require.NotNil(t, result.Undo)
	assert.Equal(t, toolUncordon, result.Undo.Tool)

	got, err := c.CoreV1().Nodes().Get(context.Background(), "a", metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, got.Spec.Unschedulable)

	_, err = e.Plan(context.Background(), request(toolCordon, map[string]any{"name": "b"}))
	require.ErrorContains(t, err, "last schedulable node")

	_, result = planAndExecute(t, c, e, request(toolUncordon, map[string]any{"name": "a"}))
	assert.Equal(t, toolCordon, result.Undo.Tool)
}

func TestSuspendAndResumeCronJob(t *testing.T) {
	cj := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Namespace: "payments", Name: "nightly"}}
	c := newCluster(t, cj)
	e := engine(t, c, ToolsConfig{})
	params := map[string]any{"namespace": "payments", "name": "nightly"}

	_, result := planAndExecute(t, c, e, request(toolSuspendCronJob, params))
	assert.Equal(t, toolResumeCronJob, result.Undo.Tool)
	got, err := c.BatchV1().CronJobs("payments").Get(context.Background(), "nightly", metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, *got.Spec.Suspend)

	_, err = e.Plan(context.Background(), request(toolSuspendCronJob, params))
	require.ErrorContains(t, err, "already suspended")

	_, result = planAndExecute(t, c, e, request(toolResumeCronJob, params))
	assert.Equal(t, toolSuspendCronJob, result.Undo.Tool)
}

// Whatever the verb, the engine only ever reads and patches, and never
// touches Secrets, Pods or RBAC.
func TestEngineOnlyReadsAndPatches(t *testing.T) {
	dep := deployment("payments", "api", 3)
	dep.Annotations = map[string]string{revisionAnnotation: "2"}
	c := newCluster(t, dep, replicaSet(dep, "1", "app:v1"), replicaSet(dep, "2", "app:v2"),
		node("a", false), node("b", false), &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Namespace: "payments", Name: "nightly"}})
	e := engine(t, c, ToolsConfig{})

	for _, req := range []pluginsdk.ToolRequest{
		request(toolScale, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": int64(4)}),
		request(toolRolloutRestart, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api"}),
		request(toolRolloutUndo, map[string]any{"namespace": "payments", "name": "api"}),
		request(toolCordon, map[string]any{"name": "a"}),
		request(toolSuspendCronJob, map[string]any{"namespace": "payments", "name": "nightly"}),
	} {
		planAndExecute(t, c, e, req)
	}

	allowed := map[string]bool{"get": true, "list": true, "patch": true}
	for _, a := range c.Actions() {
		assert.True(t, allowed[a.GetVerb()], "verb %s on %s", a.GetVerb(), a.GetResource().Resource)
		assert.NotContains(t, []string{"secrets", "pods", "roles", "rolebindings", "clusterroles", "clusterrolebindings"}, a.GetResource().Resource)
		assert.Empty(t, a.GetSubresource())
	}
}

func TestToolClientUserAgent(t *testing.T) {
	var mu sync.Mutex
	var agent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		agent = r.UserAgent()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"Node","apiVersion":"v1","metadata":{"name":"a"}}`))
	}))
	defer srv.Close()

	client, err := newToolClient(&Config{Host: srv.URL, Token: "t"}, "task_run:42")
	require.NoError(t, err)
	_, err = client.CoreV1().Nodes().Get(context.Background(), "a", metav1.GetOptions{})
	require.NoError(t, err)
	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, agent, "origin task_run:42")
}

func TestToolClientRefusesImpersonation(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(kubeconfig, []byte(`apiVersion: v1
kind: Config
clusters: [{name: c, cluster: {server: "https://127.0.0.1:6443"}}]
users: [{name: u, user: {token: t, as: cluster-admin}}]
contexts: [{name: x, context: {cluster: c, user: u}}]
current-context: x
`), 0o600))

	_, err := newToolClient(&Config{KubeconfigPath: kubeconfig}, "task_run:42")
	require.ErrorContains(t, err, "impersonates")
}

// The Source, over the SDK's own checks, refuses out of bounds parameters
// before the engine ever runs; this covers the config wiring end to end.
func TestSourceToolsUseConfig(t *testing.T) {
	s := &Source{client: newCluster(t, deployment("payments", "api", 3))}
	raw := pluginsdk.RawConfig{"tools": map[string]any{"max_replicas": 4}}
	_, err := s.PlanTool(context.Background(), raw,
		request(toolScale, map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": int64(5)}))
	require.ErrorContains(t, err, "max_replicas of 4")

	_, err = s.Validate(pluginsdk.RawConfig{"tools": map[string]any{"selector": "=bad"}})
	require.Error(t, err)
}

func TestDiffObjects(t *testing.T) {
	before := deployment("payments", "api", 3)
	after := before.DeepCopy()
	after.Spec.Replicas = ptr.To(int32(1))
	after.ResourceVersion = "99"
	after.Annotations = map[string]string{"marmot.io/origin": "task_run:1"}

	b, a, diff, err := diffObjects(before, after)
	require.NoError(t, err)
	assert.JSONEq(t, `{"spec":{"replicas":3}}`, string(b))
	assert.JSONEq(t, `{"metadata":{"annotations":{"marmot.io/origin":"task_run:1"}},"spec":{"replicas":1}}`, string(a))
	assert.Equal(t, "+ metadata.annotations[\"marmot.io/origin\"]: \"task_run:1\"\n- spec.replicas: 3\n+ spec.replicas: 1\n", diff)
	assert.False(t, errors.Is(err, ErrStale))
}
