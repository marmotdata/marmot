package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8s "k8s.io/client-go/kubernetes"
)

const (
	// fieldManager owns every field a tool writes, so server-side
	// apply and managedFields show which changes came from Marmot.
	fieldManager = "marmot"
	// The two stamps every tool leaves on the object it changes.
	annotationOrigin = "marmot.io/origin"
	annotationActor  = "marmot.io/actor"

	restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"
	revisionAnnotation    = "deployment.kubernetes.io/revision"
)

var (
	// ErrStale is returned when the object changed after it was planned.
	ErrStale = errors.New("the object changed since the tool was planned; plan it again")
	// ErrGitOpsManaged is returned for an object Flux or Argo CD manages.
	ErrGitOpsManaged = errors.New("managed by GitOps")
	// ErrOutOfScope is returned for an object the runner's tools config
	// does not allow.
	ErrOutOfScope = errors.New("outside the tools scope")
	// ErrLastNode refuses a cordon that would leave no node new pods can land on.
	ErrLastNode = errors.New("the last schedulable node")
)

// ToolEngine plans and executes the plugin's tools against one cluster.
// It is exported so the cloud plugins (eks, gke) can offer the same tools
// with their own authentication, as they reuse Discoverer.
type ToolEngine struct {
	client   k8s.Interface
	config   ToolsConfig
	selector labels.Selector
	now      func() time.Time
}

// NewToolEngine returns an engine bound by config.
func NewToolEngine(client k8s.Interface, config ToolsConfig) (*ToolEngine, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	selector, err := labels.Parse(config.Selector)
	if err != nil {
		return nil, err
	}
	return &ToolEngine{client: client, config: config, selector: selector, now: time.Now}, nil
}

// object is what every target kind's typed client returns.
type object interface {
	metav1.Object
	runtime.Object
}

// target is one object a tool changes, with the only two calls an
// tool ever makes on it.
type target struct {
	kind  pluginsdk.TargetKind
	label string
	get   func(ctx context.Context) (object, error)
	patch func(ctx context.Context, data []byte, dryRun bool) (object, error)
}

// getPatcher is the slice of a typed client a target needs: there is
// deliberately no create, update or delete in it.
type getPatcher[T object] interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (T, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error)
}

func newTarget[T object](kind pluginsdk.TargetKind, label, name string, c getPatcher[T]) *target {
	return &target{
		kind:  kind,
		label: label,
		get: func(ctx context.Context) (object, error) {
			return c.Get(ctx, name, metav1.GetOptions{})
		},
		patch: func(ctx context.Context, data []byte, dryRun bool) (object, error) {
			opts := metav1.PatchOptions{FieldManager: fieldManager}
			if dryRun {
				opts.DryRun = []string{metav1.DryRunAll}
			}
			return c.Patch(ctx, name, types.JSONPatchType, data, opts)
		},
	}
}

// resolve maps a request onto its target, refusing a namespace outside the
// tools scope before anything is read from it.
func (e *ToolEngine) resolve(req pluginsdk.ToolRequest) (*target, error) {
	ns, name := req.StringParam("namespace"), req.StringParam("name")

	kind := req.StringParam("kind")
	switch req.Tool {
	case toolRolloutUndo:
		kind = "deployment"
	case toolCordon, toolUncordon:
		kind = "node"
	case toolSuspendCronJob, toolResumeCronJob:
		kind = "cronjob"
	}

	if kind != "node" && !e.config.inScope(ns) {
		return nil, fmt.Errorf("namespace %q is %w: allow it in tools.namespaces", ns, ErrOutOfScope)
	}

	label := kind + "/" + ns + "/" + name
	switch kind {
	case "deployment":
		return newTarget(kindDeployment, label, name, e.client.AppsV1().Deployments(ns)), nil
	case "statefulset":
		return newTarget(kindStatefulSet, label, name, e.client.AppsV1().StatefulSets(ns)), nil
	case "daemonset":
		return newTarget(kindDaemonSet, label, name, e.client.AppsV1().DaemonSets(ns)), nil
	case "cronjob":
		return newTarget(kindCronJob, label, name, e.client.BatchV1().CronJobs(ns)), nil
	case "node":
		return newTarget(kindNode, "node/"+name, name, e.client.CoreV1().Nodes()), nil
	}
	return nil, fmt.Errorf("tool %s does not apply to kind %q", req.Tool, kind)
}

// admit refuses an object the runner's config does not let tools touch.
func (e *ToolEngine) admit(t *target, obj object) error {
	if !e.selector.Matches(labels.Set(obj.GetLabels())) {
		return fmt.Errorf("%s does not match tools.selector %q: %w", t.label, e.config.Selector, ErrOutOfScope)
	}
	if e.config.AllowGitOpsManaged {
		return nil
	}
	if manager, key, ok := gitOpsManager(obj); ok {
		return fmt.Errorf("%s is %w by %s (%s): make the change in Git, or the next reconcile may revert it. Set tools.allow_gitops_managed to act on it anyway",
			t.label, ErrGitOpsManaged, manager, key)
	}
	return nil
}

// gitOpsPrefixes are the Flux label and annotation prefixes, and
// gitOpsKeys Argo CD's. app.kubernetes.io/instance is deliberately absent:
// Argo CD can track by it, but every Helm chart sets it too.
var (
	gitOpsPrefixes = map[string]string{
		"kustomize.toolkit.fluxcd.io/": "Flux",
		"helm.toolkit.fluxcd.io/":      "Flux",
	}
	gitOpsKeys = map[string]string{
		"argocd.argoproj.io/instance":    "Argo CD",
		"argocd.argoproj.io/tracking-id": "Argo CD",
	}
)

func gitOpsManager(obj metav1.Object) (manager, key string, ok bool) {
	for _, m := range []map[string]string{obj.GetLabels(), obj.GetAnnotations()} {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if manager, ok := gitOpsKeys[k]; ok {
				return manager, k, true
			}
			for prefix, manager := range gitOpsPrefixes {
				if strings.HasPrefix(k, prefix) {
					return manager, k, true
				}
			}
		}
	}
	return "", "", false
}

// change is what one verb does to the object it was given.
type change struct {
	ops      []patchOp
	summary  string
	warnings []string
	// undo is the inverse request, filled from the state before the change.
	undo *pluginsdk.ToolRequest
}

// patchOp is one JSON patch operation. Value is never omitted, since false
// and 0 are values a tool writes.
type patchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// Plan previews the tool with a server-side dry run, so admission
// webhooks and defaulting have their say exactly as they would for real,
// and nothing is persisted for any input.
func (e *ToolEngine) Plan(ctx context.Context, req pluginsdk.ToolRequest) (*pluginsdk.ToolPlan, error) {
	t, obj, ch, err := e.prepare(ctx, req)
	if err != nil {
		return nil, err
	}

	after, err := t.patch(ctx, e.patchFor(obj, obj.GetResourceVersion(), ch, req), true)
	if err != nil {
		return nil, fmt.Errorf("dry run of %s on %s: %w", req.Tool, t.label, err)
	}
	before, afterJSON, diff, err := diffObjects(obj, after)
	if err != nil {
		return nil, err
	}

	return &pluginsdk.ToolPlan{
		Target:       t.label,
		TargetKind:   t.kind,
		Summary:      ch.summary,
		Before:       before,
		After:        afterJSON,
		Diff:         diff,
		Precondition: obj.GetResourceVersion(),
		Warnings:     ch.warnings,
	}, nil
}

// Execute applies the tool only if the object is still at the
// resourceVersion its plan saw. The check is made twice: once here for a
// clear error, and again inside the patch as a JSON patch test, which the
// API server applies atomically, so a change landing in between is refused
// too.
func (e *ToolEngine) Execute(ctx context.Context, req pluginsdk.ToolRequest) (*pluginsdk.ToolResult, error) {
	if isRead(req.Tool) {
		return e.read(ctx, req)
	}
	if req.Precondition == "" {
		return nil, fmt.Errorf("executing %s needs the precondition from its plan", req.Tool)
	}
	t, obj, ch, err := e.prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	if obj.GetResourceVersion() != req.Precondition {
		return nil, fmt.Errorf("%s: %w (planned at resourceVersion %s, now %s)", t.label, ErrStale, req.Precondition, obj.GetResourceVersion())
	}

	after, err := t.patch(ctx, e.patchFor(obj, req.Precondition, ch, req), false)
	if err != nil {
		return nil, fmt.Errorf("%s on %s was refused, and if the object changed since the plan, plan it again: %w", req.Tool, t.label, err)
	}
	before, afterJSON, _, err := diffObjects(obj, after)
	if err != nil {
		// The change has happened; failing now would report it as not.
		before, afterJSON = nil, nil
	}

	return &pluginsdk.ToolResult{
		Target:     t.label,
		TargetKind: t.kind,
		Summary:    ch.summary,
		Before:     before,
		After:      afterJSON,
		Undo:       ch.undo,
		Warnings:   ch.warnings,
	}, nil
}

func (e *ToolEngine) prepare(ctx context.Context, req pluginsdk.ToolRequest) (*target, object, *change, error) {
	// Serve has already checked the request, but the engine is exported for
	// the cloud plugins, so it does not rely on being called through Serve.
	if _, err := pluginsdk.CheckToolRequest(ToolSpecs(), req, false); err != nil {
		return nil, nil, nil, err
	}
	t, err := e.resolve(req)
	if err != nil {
		return nil, nil, nil, err
	}
	obj, err := t.get(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reading %s: %w", t.label, err)
	}
	if err := e.admit(t, obj); err != nil {
		return nil, nil, nil, err
	}

	var ch *change
	switch req.Tool {
	case toolScale:
		ch, err = e.scale(ctx, t, obj, req)
	case toolRolloutRestart:
		ch, err = e.restart(t, obj)
	case toolRolloutUndo:
		ch, err = e.rollbackDeployment(ctx, t, obj.(*appsv1.Deployment), req)
	case toolCordon, toolUncordon:
		ch, err = e.cordon(ctx, t, obj.(*corev1.Node), req.Tool == toolCordon)
	case toolSuspendCronJob, toolResumeCronJob:
		ch, err = suspendCronJob(t, obj.(*batchv1.CronJob), req.Tool == toolSuspendCronJob)
	default:
		err = fmt.Errorf("unknown tool %q", req.Tool)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if ref := metav1.GetControllerOf(obj); ref != nil {
		ch.warnings = append(ch.warnings, fmt.Sprintf("%s is controlled by %s %s, which may revert this change", t.label, ref.Kind, ref.Name))
	}
	return t, obj, ch, nil
}

// patchFor wraps a verb's operations in the resourceVersion test and the
// origin and actor stamps.
func (e *ToolEngine) patchFor(obj object, resourceVersion string, ch *change, req pluginsdk.ToolRequest) []byte {
	ops := []patchOp{{Op: "test", Path: "/metadata/resourceVersion", Value: resourceVersion}}
	ops = append(ops, ch.ops...)
	ops = append(ops, annotationOps("/metadata/annotations", obj.GetAnnotations(), map[string]string{
		annotationOrigin: req.Origin,
		annotationActor:  req.Identity,
	})...)
	data, err := json.Marshal(ops)
	if err != nil {
		// Only strings, bools, ints and API types go in.
		panic(fmt.Sprintf("encoding a JSON patch: %v", err))
	}
	return data
}

// annotationOps sets annotations under path. JSON patch cannot add a key to
// a map that is absent, so a missing map is added whole.
func annotationOps(path string, existing, set map[string]string) []patchOp {
	if existing == nil {
		return []patchOp{{Op: "add", Path: path, Value: set}}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ops := make([]patchOp, 0, len(keys))
	for _, k := range keys {
		ops = append(ops, patchOp{Op: "add", Path: path + "/" + escapePointer(k), Value: set[k]})
	}
	return ops
}

// escapePointer escapes a JSON pointer segment (RFC 6901).
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func (e *ToolEngine) scale(ctx context.Context, t *target, obj object, req pluginsdk.ToolRequest) (*change, error) {
	replicas, _ := req.IntParam("replicas")
	if limit := e.config.maxReplicas(); replicas > limit {
		return nil, fmt.Errorf("%d replicas is above this runner's tools.max_replicas of %d", replicas, limit)
	}

	var current *int32
	var kind string
	switch o := obj.(type) {
	case *appsv1.Deployment:
		current, kind = o.Spec.Replicas, "Deployment"
	case *appsv1.StatefulSet:
		current, kind = o.Spec.Replicas, "StatefulSet"
	default:
		return nil, fmt.Errorf("%s cannot be scaled", t.label)
	}
	// An unset replica count is defaulted to 1 by the API server.
	previous := int64(1)
	if current != nil {
		previous = int64(*current)
	}
	if previous == replicas {
		return nil, fmt.Errorf("%s already has %d replicas", t.label, replicas)
	}

	ch := &change{
		ops:     []patchOp{{Op: "add", Path: "/spec/replicas", Value: replicas}},
		summary: fmt.Sprintf("Scale %s from %d to %d replicas", t.label, previous, replicas),
		undo: &pluginsdk.ToolRequest{Tool: toolScale, Params: map[string]any{
			"kind": req.StringParam("kind"), "namespace": obj.GetNamespace(), "name": obj.GetName(), "replicas": previous,
		}},
	}
	if replicas == 0 {
		ch.warnings = append(ch.warnings, fmt.Sprintf("scaling to zero stops every pod of %s", t.label))
	}
	ch.warnings = append(ch.warnings, e.autoscalerWarnings(ctx, obj.GetNamespace(), kind, obj.GetName())...)
	return ch, nil
}

// autoscalerWarnings flags a HorizontalPodAutoscaler that would undo a
// scale. It is advice only: a runner without access to autoscalers is told
// it could not check rather than refused.
func (e *ToolEngine) autoscalerWarnings(ctx context.Context, namespace, kind, name string) []string {
	list, err := e.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{fmt.Sprintf("could not check for a HorizontalPodAutoscaler: %v", err)}
	}
	var warnings []string
	for _, hpa := range list.Items {
		ref := hpa.Spec.ScaleTargetRef
		if ref.Kind == kind && ref.Name == name {
			warnings = append(warnings, fmt.Sprintf("HorizontalPodAutoscaler %s manages these replicas and will override this", hpa.Name))
		}
	}
	return warnings
}

func (e *ToolEngine) restart(t *target, obj object) (*change, error) {
	var template corev1.PodTemplateSpec
	var warnings []string
	switch o := obj.(type) {
	case *appsv1.Deployment:
		// kubectl refuses too: a paused Deployment would not roll.
		if o.Spec.Paused {
			return nil, fmt.Errorf("%s is paused; resume it before restarting", t.label)
		}
		template = o.Spec.Template
	case *appsv1.StatefulSet:
		template = o.Spec.Template
		if o.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType {
			warnings = append(warnings, "the update strategy is OnDelete, so pods restart only when deleted")
		}
	case *appsv1.DaemonSet:
		template = o.Spec.Template
		if o.Spec.UpdateStrategy.Type == appsv1.OnDeleteDaemonSetStrategyType {
			warnings = append(warnings, "the update strategy is OnDelete, so pods restart only when deleted")
		}
	default:
		return nil, fmt.Errorf("%s cannot be restarted", t.label)
	}

	return &change{
		ops: annotationOps("/spec/template/metadata/annotations", template.Annotations, map[string]string{
			restartedAtAnnotation: e.now().UTC().Format(time.RFC3339),
		}),
		summary:  fmt.Sprintf("Restart every pod of %s with a rolling update", t.label),
		warnings: warnings,
	}, nil
}

// rollbackDeployment does what kubectl rollout undo does: find the
// ReplicaSet of the wanted revision among those the Deployment controls and
// copy its pod template back. ReplicaSets are read, never written.
func (e *ToolEngine) rollbackDeployment(ctx context.Context, t *target, dep *appsv1.Deployment, req pluginsdk.ToolRequest) (*change, error) {
	if dep.Spec.Paused {
		return nil, fmt.Errorf("%s is paused; resume it before rolling back", t.label)
	}
	current, err := strconv.ParseInt(dep.Annotations[revisionAnnotation], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s has no revision yet, so there is nothing to roll back to", t.label)
	}

	selector, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("%s has an invalid selector: %w", t.label, err)
	}
	list, err := e.client.AppsV1().ReplicaSets(dep.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, fmt.Errorf("listing the revisions of %s: %w", t.label, err)
	}

	revisions := map[int64]*appsv1.ReplicaSet{}
	for i := range list.Items {
		rs := &list.Items[i]
		if ref := metav1.GetControllerOf(rs); ref == nil || ref.UID != dep.UID {
			continue
		}
		if rev, err := strconv.ParseInt(rs.Annotations[revisionAnnotation], 10, 64); err == nil {
			revisions[rev] = rs
		}
	}

	want, explicit := req.IntParam("to_revision")
	if !explicit {
		want = 0
		for rev := range revisions {
			if rev < current && rev > want {
				want = rev
			}
		}
		if want == 0 {
			return nil, fmt.Errorf("%s has no revision before %d to roll back to", t.label, current)
		}
	}
	if want == current {
		return nil, fmt.Errorf("revision %d is already the current one of %s", want, t.label)
	}
	rs, ok := revisions[want]
	if !ok {
		return nil, fmt.Errorf("%s has no revision %d; the history keeps %d", t.label, want, len(revisions))
	}

	template := *rs.Spec.Template.DeepCopy()
	delete(template.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	if apiequality.Semantic.DeepEqual(template, dep.Spec.Template) {
		return nil, fmt.Errorf("the pod template of %s already matches revision %d", t.label, want)
	}

	return &change{
		ops:     []patchOp{{Op: "add", Path: "/spec/template", Value: template}},
		summary: fmt.Sprintf("Roll %s back from revision %d to revision %d", t.label, current, want),
		// The controller renumbers the restored ReplicaSet as the newest
		// revision and leaves the one replaced here at current, so rolling
		// back to current is the exact inverse.
		undo: &pluginsdk.ToolRequest{Tool: toolRolloutUndo, Params: map[string]any{
			"namespace": dep.Namespace, "name": dep.Name, "to_revision": current,
		}},
	}, nil
}

func (e *ToolEngine) cordon(ctx context.Context, t *target, node *corev1.Node, cordon bool) (*change, error) {
	if node.Spec.Unschedulable == cordon {
		state := "schedulable"
		if cordon {
			state = "cordoned"
		}
		return nil, fmt.Errorf("%s is already %s", t.label, state)
	}

	ch := &change{
		ops:     []patchOp{{Op: "add", Path: "/spec/unschedulable", Value: cordon}},
		summary: fmt.Sprintf("Uncordon %s so pods can be scheduled on it again", t.label),
		undo:    &pluginsdk.ToolRequest{Tool: toolCordon, Params: map[string]any{"name": node.Name}},
	}
	if !cordon {
		return ch, nil
	}

	ch.summary = fmt.Sprintf("Cordon %s so no new pods are scheduled on it; running pods stay", t.label)
	ch.undo = &pluginsdk.ToolRequest{Tool: toolUncordon, Params: map[string]any{"name": node.Name}}
	if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
		ch.warnings = append(ch.warnings, fmt.Sprintf("%s is a control plane node", t.label))
	}

	// Not knowing how many nodes remain is a reason to refuse, not to go ahead.
	nodes, err := e.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("counting the schedulable nodes before cordoning %s: %w", t.label, err)
	}
	schedulable := 0
	for i := range nodes.Items {
		if n := &nodes.Items[i]; n.Name != node.Name && takesPods(n) {
			schedulable++
		}
	}
	// Cordoning the last node leaves nowhere for anything to be scheduled,
	// which no single bounded tool should be able to do.
	if schedulable == 0 {
		return nil, fmt.Errorf("%s is %w", t.label, ErrLastNode)
	}
	ch.warnings = append(ch.warnings, fmt.Sprintf("%d other nodes stay schedulable", schedulable))
	return ch, nil
}

func suspendCronJob(t *target, cj *batchv1.CronJob, suspend bool) (*change, error) {
	suspended := cj.Spec.Suspend != nil && *cj.Spec.Suspend
	if suspended == suspend {
		state := "running"
		if suspend {
			state = "suspended"
		}
		return nil, fmt.Errorf("%s is already %s", t.label, state)
	}

	params := map[string]any{"namespace": cj.Namespace, "name": cj.Name}
	if suspend {
		return &change{
			ops:     []patchOp{{Op: "add", Path: "/spec/suspend", Value: true}},
			summary: fmt.Sprintf("Suspend %s so it starts no new runs", t.label),
			undo:    &pluginsdk.ToolRequest{Tool: toolResumeCronJob, Params: params},
		}, nil
	}
	return &change{
		ops:      []patchOp{{Op: "add", Path: "/spec/suspend", Value: false}},
		summary:  fmt.Sprintf("Resume %s", t.label),
		warnings: []string{"a run missed while suspended may start straight away, within startingDeadlineSeconds"},
		undo:     &pluginsdk.ToolRequest{Tool: toolSuspendCronJob, Params: params},
	}, nil
}

// ignoredPaths change on every write and say nothing about the tool.
var ignoredPaths = [][]string{
	{"metadata", "resourceVersion"},
	{"metadata", "generation"},
	{"metadata", "managedFields"},
	{"status"},
}

// diffObjects returns the fields that differ between two versions of an
// object, as before and after JSON holding only those fields, and a
// readable diff. Lists are compared whole, since their elements have no
// stable path.
func diffObjects(before, after runtime.Object) (json.RawMessage, json.RawMessage, string, error) {
	b, err := runtime.DefaultUnstructuredConverter.ToUnstructured(before)
	if err != nil {
		return nil, nil, "", fmt.Errorf("converting the object: %w", err)
	}
	a, err := runtime.DefaultUnstructuredConverter.ToUnstructured(after)
	if err != nil {
		return nil, nil, "", fmt.Errorf("converting the planned object: %w", err)
	}
	for _, p := range ignoredPaths {
		removePath(b, p)
		removePath(a, p)
	}

	var changed [][]string
	collectChanges(b, a, nil, &changed)
	sort.Slice(changed, func(i, j int) bool { return renderPath(changed[i]) < renderPath(changed[j]) })

	beforeOut, afterOut := map[string]any{}, map[string]any{}
	var diff strings.Builder
	for _, p := range changed {
		if v, ok := lookup(b, p); ok {
			setPath(beforeOut, p, v)
			fmt.Fprintf(&diff, "- %s: %s\n", renderPath(p), renderValue(v))
		}
		if v, ok := lookup(a, p); ok {
			setPath(afterOut, p, v)
			fmt.Fprintf(&diff, "+ %s: %s\n", renderPath(p), renderValue(v))
		}
	}

	beforeJSON, err := json.Marshal(beforeOut)
	if err != nil {
		return nil, nil, "", err
	}
	afterJSON, err := json.Marshal(afterOut)
	if err != nil {
		return nil, nil, "", err
	}
	return beforeJSON, afterJSON, diff.String(), nil
}

func collectChanges(b, a map[string]any, prefix []string, out *[][]string) {
	keys := map[string]bool{}
	for k := range b {
		keys[k] = true
	}
	for k := range a {
		keys[k] = true
	}
	for k := range keys {
		path := append(slices.Clone(prefix), k)
		bv, bok := b[k]
		av, aok := a[k]
		bm, bIsMap := bv.(map[string]any)
		am, aIsMap := av.(map[string]any)
		// A map that appears or disappears is walked as if empty on the
		// other side, so the diff names the keys rather than the whole map.
		switch {
		case bIsMap && !aok:
			collectChanges(bm, map[string]any{}, path, out)
		case aIsMap && !bok:
			collectChanges(map[string]any{}, am, path, out)
		case bIsMap && aIsMap:
			collectChanges(bm, am, path, out)
		case bok != aok || !reflect.DeepEqual(bv, av):
			*out = append(*out, path)
		}
	}
}

func lookup(m map[string]any, path []string) (any, bool) {
	var cur any = m
	for _, k := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = mm[k]; !ok {
			return nil, false
		}
	}
	return cur, true
}

func setPath(m map[string]any, path []string, v any) {
	for _, k := range path[:len(path)-1] {
		next, ok := m[k].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[k] = next
		}
		m = next
	}
	m[path[len(path)-1]] = v
}

func removePath(m map[string]any, path []string) {
	for _, k := range path[:len(path)-1] {
		next, ok := m[k].(map[string]any)
		if !ok {
			return
		}
		m = next
	}
	delete(m, path[len(path)-1])
}

// renderPath writes spec.template.metadata.annotations["a/b"], quoting the
// keys a dotted path would misread.
func renderPath(path []string) string {
	var b strings.Builder
	for i, k := range path {
		if strings.ContainsAny(k, "./") {
			fmt.Fprintf(&b, "[%q]", k)
			continue
		}
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(k)
	}
	return b.String()
}

func renderValue(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}

// takesPods reports whether ordinary pods can be scheduled on n: Ready, not cordoned, and with no NoSchedule or NoExecute taint. A control plane node is usually tainted, so it does not count as somewhere a workload can go, even though it is not cordoned.
func takesPods(n *corev1.Node) bool {
	if n.Spec.Unschedulable {
		return false
	}
	for _, taint := range n.Spec.Taints {
		if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
			return false
		}
	}
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
