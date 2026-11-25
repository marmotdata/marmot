package kubernetes

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// recorder collects what a watch emits, checked by the SDK exactly as Serve checks it.
type recorder struct {
	t      *testing.T
	mu     sync.Mutex
	events []pluginsdk.TriggerEvent
}

func (r *recorder) emit(spec string) func(pluginsdk.TriggerEvent) error {
	return func(ev pluginsdk.TriggerEvent) error {
		r.t.Helper()
		for _, s := range TriggerSpecs() {
			if s.Name == spec {
				checked, err := pluginsdk.CheckTriggerEvent(s, ev)
				require.NoError(r.t, err)
				ev = checked
			}
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		r.events = append(r.events, ev)
		return nil
	}
}

func (r *recorder) take() []pluginsdk.TriggerEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.events
	r.events = nil
	return out
}

// startWatch builds a watch on c and starts its informers, without the evaluation loop, so a test drives the clock.
func startWatch(t *testing.T, c *fake.Clientset, config ToolsConfig, trigger string, params map[string]any) (*TriggerWatch, *recorder) {
	t.Helper()
	r := &recorder{t: t}
	w, err := NewTriggerWatch(c, config, trigger, params, r.emit(trigger))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, w.checkAccess(ctx))
	require.NoError(t, w.start(ctx))
	return w, r
}

// eventually re-evaluates until the informers have caught up with a change.
func eventually(t *testing.T, w *TriggerWatch, r *recorder, at time.Time, want int) []pluginsdk.TriggerEvent {
	t.Helper()
	var got []pluginsdk.TriggerEvent
	require.Eventually(t, func() bool {
		require.NoError(t, w.evaluate(at))
		got = append(got, r.take()...)
		return len(got) >= want
	}, 3*time.Second, 10*time.Millisecond)
	return got
}

func TestTriggerManifestIsValid(t *testing.T) {
	require.NoError(t, pluginsdk.ValidateTriggers(TriggerSpecs()))
}

func crashyCluster(t *testing.T) *fake.Clientset {
	crashy, api := deployment("payments", "crashy", 1), deployment("payments", "api", 1)
	return fake.NewClientset(crashy, api,
		ownedReplicaSet("payments", "crashy", "abc12"), ownedReplicaSet("payments", "api", "abc12"),
		crashingPod("payments", "crashy", "a", 6), healthyPod("payments", "api", "a"))
}

func TestWorkloadUnhealthyCrashLooping(t *testing.T) {
	c := crashyCluster(t)
	w, r := startWatch(t, c, ToolsConfig{Namespaces: []string{"payments"}}, triggerWorkloadUnhealthy,
		map[string]any{"conditions": []any{"crash_looping"}, "for_minutes": 1})
	t0 := testNow

	require.NoError(t, w.evaluate(t0))
	assert.Empty(t, r.take(), "not yet held for a minute")

	events := eventually(t, w, r, t0.Add(61*time.Second), 1)
	require.Len(t, events, 1, "crashy only; api is healthy")
	ev := events[0]
	assert.Equal(t, pluginsdk.TriggerTarget{Kind: kindDeployment, Namespace: "payments", Name: "crashy"}, ev.Target, "a pod's condition is reported against its workload")
	assert.Equal(t, "crash_looping", ev.Facts["condition"])
	assert.Equal(t, "deployment/crashy", ev.Facts["workload"])
	assert.Equal(t, "deployment", ev.Facts["kind"])
	assert.Equal(t, int64(1), ev.Facts["pods_affected"])
	assert.Equal(t, int64(6), ev.Facts["restart_count"])
	assert.Equal(t, "CrashLoopBackOff", ev.Facts["reason"])
	assert.Equal(t, t0.Format(time.RFC3339), ev.Facts["since"])

	require.NoError(t, w.evaluate(t0.Add(5*time.Minute)))
	assert.Empty(t, r.take(), "one event while it holds")

	// It clears, then comes back and fires again once it has held again.
	fixed := healthyPod("payments", "crashy", "a")
	_, err := c.CoreV1().Pods("payments").UpdateStatus(context.Background(), fixed, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool { require.NoError(t, w.evaluate(t0.Add(6*time.Minute))); return len(w.state) == 0 }, 3*time.Second, 10*time.Millisecond)
	_, err = c.CoreV1().Pods("payments").UpdateStatus(context.Background(), crashingPod("payments", "crashy", "a", 7), metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool { require.NoError(t, w.evaluate(t0.Add(7*time.Minute))); return len(w.state) == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.Empty(t, r.take())
	again := eventually(t, w, r, t0.Add(8*time.Minute+time.Second), 1)
	assert.Equal(t, ev.Fingerprint, again[0].Fingerprint, "the fingerprint is stable per trigger, condition and target")
}

func TestWorkloadUnhealthyConditions(t *testing.T) {
	pull := crashingPod("payments", "puller", "a", 0)
	pull.Status.ContainerStatuses[0].State.Waiting.Reason = "ImagePullBackOff"
	pull.Status.ContainerStatuses[0].LastTerminationState = corev1.ContainerState{}
	pull.Status.Phase = corev1.PodPending
	pending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "payments", Name: "lonely", Labels: map[string]string{"app": "lonely"}},
		Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{
			{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable},
		}},
	}
	stuck := deployment("payments", "stuck", 3)
	stuck.Status = appsv1.DeploymentStatus{AvailableReplicas: 1, Conditions: []appsv1.DeploymentCondition{
		{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"},
	}}
	c := fake.NewClientset(deployment("payments", "puller", 1), ownedReplicaSet("payments", "puller", "abc12"), pull, pending, stuck)
	w, r := startWatch(t, c, ToolsConfig{}, triggerWorkloadUnhealthy, map[string]any{"namespaces": []any{"payments"}, "for_minutes": 1})
	require.NoError(t, w.evaluate(testNow))
	events := eventually(t, w, r, testNow.Add(2*time.Minute), 5)
	got := map[string]bool{}
	for _, ev := range events {
		got[ev.Facts["condition"].(string)+" "+string(ev.Target.Kind)+"/"+ev.Target.Name] = true
	}
	assert.Equal(t, map[string]bool{
		"image_pull_failing kubernetes/deployment/puller":   true,
		"replicas_unavailable kubernetes/deployment/puller": true,
		// A pod with no workload is its own target.
		"unschedulable kubernetes/pod/lonely":              true,
		"rollout_stuck kubernetes/deployment/stuck":        true,
		"replicas_unavailable kubernetes/deployment/stuck": true,
	}, got)
}

func TestTriggerScope(t *testing.T) {
	c := crashyCluster(t)
	_, err := NewTriggerWatch(c, ToolsConfig{Namespaces: []string{"payments"}}, triggerWorkloadUnhealthy, map[string]any{"namespaces": []any{"internal"}}, nil)
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "a namespace outside tools.namespaces")

	_, err = NewTriggerWatch(c, ToolsConfig{}, triggerWorkloadUnhealthy, map[string]any{"for_minutes": 0}, nil)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	// A selector the workload does not carry keeps it unreported.
	w, r := startWatch(t, c, ToolsConfig{Selector: "marmot.io/tools=enabled"}, triggerWorkloadUnhealthy, map[string]any{"for_minutes": 1})
	require.NoError(t, w.evaluate(testNow))
	require.NoError(t, w.evaluate(testNow.Add(2*time.Minute)))
	assert.Empty(t, r.take())

	// Missing RBAC is a clear refusal before any watch starts.
	denied := fake.NewClientset()
	denied.PrependReactor("list", "replicasets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "replicasets"}, "", errors.New("denied"))
	})
	dw, err := NewTriggerWatch(denied, ToolsConfig{Namespaces: []string{"payments"}}, triggerWorkloadUnhealthy, nil, nil)
	require.NoError(t, err)
	err = dw.checkAccess(context.Background())
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.ErrorContains(t, err, "triggers.enabled")
}

func TestNodeUnhealthy(t *testing.T) {
	notReady := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n1"}, Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionFalse, Reason: "KubeletNotReady"},
		{Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue},
	}}}
	cordoned := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n2"}, Spec: corev1.NodeSpec{Unschedulable: true},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}}}
	healthy := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n3"}, Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}}}
	c := fake.NewClientset(notReady, cordoned, healthy)
	w, r := startWatch(t, c, ToolsConfig{}, triggerNodeUnhealthy, map[string]any{"for_minutes": 5})
	require.NoError(t, w.evaluate(testNow))
	events := eventually(t, w, r, testNow.Add(5*time.Minute), 3)
	got := map[string]bool{}
	for _, ev := range events {
		got[ev.Target.Name+":"+ev.Facts["condition"].(string)] = true
		assert.NotContains(t, ev.Facts, "namespace")
	}
	assert.Equal(t, map[string]bool{"n1:not_ready": true, "n1:disk_pressure": true, "n2:unschedulable_for": true}, got)
}

func TestWarningEvents(t *testing.T) {
	ev := func(name, obj, reason string, count int32, age time.Duration) *corev1.Event {
		return &corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Namespace: "payments", Name: name},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: obj, Namespace: "payments"},
			Type:           corev1.EventTypeWarning, Reason: reason, Count: count, Message: "MountVolume.SetUp failed\x1b[0m",
			LastTimestamp: metav1.NewTime(testNow.Add(-age)),
		}
	}
	c := fake.NewClientset(
		ev("a", "db-0", "FailedMount", 2, time.Minute),
		ev("b", "db-0", "FailedMount", 2, 2*time.Minute),
		ev("c", "db-1", "FailedMount", 1, time.Minute),
		ev("d", "db-2", "FailedMount", 9, time.Hour), // outside the window
		ev("e", "db-3", "Pulled", 9, time.Minute),    // not a listed reason
	)
	w, r := startWatch(t, c, ToolsConfig{Namespaces: []string{"payments"}}, triggerWarningEvents, map[string]any{"window_minutes": 10, "min_count": 3})
	events := eventually(t, w, r, testNow, 1)
	require.Len(t, events, 1)
	assert.Equal(t, "db-0", events[0].Target.Name)
	assert.Equal(t, int64(4), events[0].Facts["count"])
	assert.Equal(t, "failed_mount", events[0].Facts["condition"])
	assert.Equal(t, "FailedMount", events[0].Facts["reason"])
	assert.Equal(t, "MountVolume.SetUp failed[0m", events[0].Detail, "the message travels only as detail")
}

func TestCustomCondition(t *testing.T) {
	below := deployment("payments", "crashy", 2)
	below.Status.AvailableReplicas = 1
	full := deployment("payments", "api", 1)
	full.Status.AvailableReplicas = 1
	// spec.replicas missing makes the expression fail on this one: it must not fire.
	broken := deployment("payments", "odd", 1)
	broken.Spec.Replicas = nil
	c := fake.NewClientset(below, full, broken)
	params := map[string]any{
		"kind": "deployment", "name": "below_desired", "for_minutes": 1,
		"when": "object.status.?availableReplicas.orValue(0) < object.spec.replicas",
	}
	w, r := startWatch(t, c, ToolsConfig{Namespaces: []string{"payments"}}, triggerCustomCondition, params)
	require.NoError(t, w.evaluate(testNow))
	events := eventually(t, w, r, testNow.Add(2*time.Minute), 1)
	require.Len(t, events, 1)
	assert.Equal(t, "crashy", events[0].Target.Name)
	assert.Equal(t, "below_desired", events[0].Facts["condition"])
	assert.Equal(t, "deployment/crashy", events[0].Facts["workload"])
	assert.Len(t, events[0].Facts, 6, "the fixed set: kind, namespace, name, workload, condition, since")

	for name, expr := range map[string]string{
		"a compile error": "object.status.availableReplicas <",
		"a non-bool":      "object.spec.replicas",
		"an unknown name": "missing.field == 1",
	} {
		p := map[string]any{"kind": "deployment", "name": "x", "when": expr}
		_, err := NewTriggerWatch(c, ToolsConfig{}, triggerCustomCondition, p, nil)
		assert.Equal(t, codes.InvalidArgument, status.Code(err), name)
	}
}

func TestCustomConditionOnPods(t *testing.T) {
	c := crashyCluster(t)
	params := map[string]any{"kind": "pod", "name": "restarting", "for_minutes": 1,
		"when": "object.status.containerStatuses.exists(s, s.restartCount > 3)"}
	w, r := startWatch(t, c, ToolsConfig{Namespaces: []string{"payments"}}, triggerCustomCondition, params)
	require.NoError(t, w.evaluate(testNow))
	events := eventually(t, w, r, testNow.Add(2*time.Minute), 1)
	require.Len(t, events, 1, "the healthy pod has no restarts")
	assert.Equal(t, kindPod, events[0].Target.Kind)
	assert.Equal(t, "deployment/crashy", events[0].Facts["workload"])
}
