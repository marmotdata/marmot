package kubernetes

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var (
	// evalInterval is how often a watch looks at what its informers hold.
	// Informers keep the view current between list and watch, so this
	// bounds only how soon a condition is noticed, and is never below 30
	// seconds outside tests.
	evalInterval = 30 * time.Second
	// informerResync replays the informers' caches; no faster than 30 s,
	// so a watch never adds load the API server would notice.
	informerResync = 5 * time.Minute
	// syncTimeout bounds the first list. Access has already been checked,
	// so this only fails on an API server too slow to list.
	syncTimeout = 2 * time.Minute
)

// maxTracked bounds how many (target, condition) pairs one watch remembers,
// so a cluster with thousands of broken pods cannot grow it without end.
const maxTracked = 5000

// observation is one condition holding on one target at one look.
type observation struct {
	target    pluginsdk.TriggerTarget
	condition string
	reason    string
	workload  string
	// count is pods_affected for a workload, or the event count for
	// warning_events; zero leaves it out.
	count        int64
	countFact    string
	restartCount int64
	detail       string
}

func (o observation) key() string {
	return pluginsdk.TriggerFingerprint(o.condition, string(o.target.Kind), o.target.Namespace, o.target.Name)
}

// held is a pair the watch has seen hold at every look since since.
type held struct {
	since time.Time
	fired bool
}

// TriggerWatch is one trigger running against one cluster. It is exported
// so the cloud plugins can offer the same triggers with their own
// authentication.
type TriggerWatch struct {
	trigger string
	client  k8s.Interface
	config  ActionsConfig
	scope   labels.Selector
	// selector is the trigger's own, on top of the actions scope.
	selector labels.Selector
	// namespaces nil means cluster wide.
	namespaces []string
	forDur     time.Duration
	emit       func(pluginsdk.TriggerEvent) error
	now        func() time.Time

	factories []informers.SharedInformerFactory
	// need lists the resources to check access to before anything starts.
	need []resourceCheck

	observe func(now time.Time) []observation
	state   map[string]*held
	capped  bool
}

type resourceCheck struct {
	what string
	list func(ctx context.Context, ns string) error
	// cluster is set for a resource that belongs to no namespace.
	cluster bool
}

// NewTriggerWatch builds the watch for trigger with params from the
// runner's policy, checking them against the actions scope. It reads
// nothing yet.
func NewTriggerWatch(client k8s.Interface, config ActionsConfig, trigger string, params map[string]any, emit func(pluginsdk.TriggerEvent) error) (*TriggerWatch, error) {
	if err := config.Validate(); err != nil {
		return nil, invalid("%v", err)
	}
	i := slices.IndexFunc(TriggerSpecs(), func(s pluginsdk.TriggerSpec) bool { return s.Name == trigger })
	if i < 0 {
		return nil, invalid("unknown trigger %q", trigger)
	}
	spec := TriggerSpecs()[i]
	// Serve has already checked them; the watch is exported, so it checks
	// again rather than trusting its caller.
	if err := pluginsdk.ValidateTriggerParams(spec, params); err != nil {
		return nil, invalid("parameters of %s: %v", trigger, err)
	}
	params = pluginsdk.NormalizeTriggerParams(spec, params)

	scope, err := labels.Parse(config.Selector)
	if err != nil {
		return nil, invalid("actions.selector: %v", err)
	}
	selector := labels.Everything()
	if s, _ := params["selector"].(string); s != "" {
		if selector, err = labels.Parse(s); err != nil {
			return nil, invalid("selector %q: %v", s, err)
		}
	}
	w := &TriggerWatch{
		trigger: trigger, client: client, config: config, scope: scope, selector: selector,
		forDur: time.Duration(intParam(params, "for_minutes", defaultForMinutes)) * time.Minute,
		emit:   emit, now: time.Now, state: map[string]*held{},
	}
	if trigger != triggerNodeUnhealthy && !(trigger == triggerCustomCondition && params["kind"] == "node") {
		named, _ := params["namespaces"].([]string)
		if w.namespaces, err = scopeNamespaces(config, named); err != nil {
			return nil, err
		}
	}

	switch trigger {
	case triggerWorkloadUnhealthy:
		err = w.setupWorkloads(params)
	case triggerNodeUnhealthy:
		err = w.setupNodes(params)
	case triggerWarningEvents:
		err = w.setupEvents(params)
	case triggerCustomCondition:
		err = w.setupCustom(params)
	}
	if err != nil {
		return nil, err
	}
	return w, nil
}

// factoriesFor returns one informer factory per watched namespace, or one
// for the cluster. Factories are per namespace because the chart grants
// per-namespace Roles, under which a cluster-wide list is forbidden.
func (w *TriggerWatch) namespacedFactories() []informers.SharedInformerFactory {
	strip := informers.WithTransform(stripManagedFields)
	if w.namespaces == nil {
		return []informers.SharedInformerFactory{informers.NewSharedInformerFactoryWithOptions(w.client, informerResync, strip)}
	}
	out := make([]informers.SharedInformerFactory, 0, len(w.namespaces))
	for _, ns := range w.namespaces {
		out = append(out, informers.NewSharedInformerFactoryWithOptions(w.client, informerResync, strip, informers.WithNamespace(ns)))
	}
	return out
}

// stripManagedFields drops managedFields from cached objects: they are the
// largest part of most objects and nothing here reads them.
func stripManagedFields(obj any) (any, error) {
	if m, err := meta.Accessor(obj); err == nil {
		m.SetManagedFields(nil)
	}
	return obj, nil
}

// Run checks access, starts the informers, waits for their first list and
// then looks every evalInterval until ctx ends. An access or sync failure
// returns before any event.
func (w *TriggerWatch) Run(ctx context.Context) error {
	if err := w.checkAccess(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := w.start(ctx); err != nil {
		return err
	}
	log.Info().Str("trigger", w.trigger).Strs("namespaces", w.namespaces).Msg("Watching")

	ticker := time.NewTicker(evalInterval)
	defer ticker.Stop()
	for {
		if err := w.evaluate(w.now()); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// checkAccess lists one item of each resource the watch needs in each
// namespace, so missing RBAC is a clear refusal now rather than an
// informer retrying forbidden lists in the background.
func (w *TriggerWatch) checkAccess(ctx context.Context) error {
	for _, r := range w.need {
		scopes := w.namespaces
		if r.cluster || scopes == nil {
			scopes = []string{metav1.NamespaceAll}
		}
		for _, ns := range scopes {
			if err := r.list(ctx, ns); err != nil {
				where := "across the cluster"
				if ns != "" {
					where = "in " + ns
				}
				if apierrors.IsForbidden(err) {
					return status.Errorf(codes.PermissionDenied, "%s may not list %s %s: install the chart with triggers.enabled, and triggers.clusterWide for cluster-wide watches and nodes", w.trigger, r.what, where)
				}
				return fmt.Errorf("%s: listing %s %s: %w", w.trigger, r.what, where, err)
			}
		}
	}
	return nil
}

func (w *TriggerWatch) start(ctx context.Context) error {
	for _, f := range w.factories {
		f.Start(ctx.Done())
	}
	syncCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	for _, f := range w.factories {
		for typ, ok := range f.WaitForCacheSync(syncCtx.Done()) {
			if !ok {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return status.Errorf(codes.Unavailable, "%s: the first list of %v did not finish", w.trigger, typ)
			}
		}
	}
	return nil
}

// evaluate looks once: every pair seen holding is remembered from the first
// look it held at, fires once it has held for forDur, and is forgotten the
// first look it does not hold, so it can fire again if it comes back. The
// runner's cooldown decides how soon a repeat reaches Marmot.
func (w *TriggerWatch) evaluate(now time.Time) error {
	obs := w.observe(now)
	sort.SliceStable(obs, func(i, j int) bool { return obs[i].key() < obs[j].key() })
	seen := make(map[string]bool, len(obs))
	for _, o := range obs {
		key := o.key()
		if seen[key] {
			continue
		}
		seen[key] = true
		h := w.state[key]
		if h == nil {
			if len(w.state) >= maxTracked {
				if !w.capped {
					log.Warn().Str("trigger", w.trigger).Int("tracked", maxTracked).Msg("Tracking as many unhealthy objects as allowed; new ones are ignored until some recover")
					w.capped = true
				}
				continue
			}
			h = &held{since: now}
			w.state[key] = h
		}
		if h.fired || now.Sub(h.since) < w.forDur {
			continue
		}
		h.fired = true
		if err := w.emit(w.event(o, h, now)); err != nil {
			if status.Code(err) == codes.Unknown && strings.HasPrefix(err.Error(), "event not sent") {
				// The SDK refused the event as malformed; that is a bug
				// here, not a reason to stop watching.
				log.Error().Err(err).Str("trigger", w.trigger).Msg("Dropped an event")
				continue
			}
			return err
		}
	}
	for key := range w.state {
		if !seen[key] {
			delete(w.state, key)
		}
	}
	if len(w.state) < maxTracked {
		w.capped = false
	}
	return nil
}

func (w *TriggerWatch) event(o observation, h *held, now time.Time) pluginsdk.TriggerEvent {
	facts := map[string]any{
		"kind":      shortKind(o.target.Kind),
		"name":      o.target.Name,
		"condition": o.condition,
		"since":     h.since.UTC().Format(time.RFC3339),
	}
	if o.target.Namespace != "" {
		facts["namespace"] = o.target.Namespace
	}
	if o.reason != "" {
		facts["reason"] = capText(o.reason, pluginsdk.MaxTriggerFactLength)
	}
	if o.workload != "" {
		facts["workload"] = o.workload
	}
	if o.countFact != "" {
		facts[o.countFact] = o.count
	}
	if o.restartCount > 0 {
		facts["restart_count"] = o.restartCount
	}
	return pluginsdk.TriggerEvent{
		Trigger:     w.trigger,
		Target:      o.target,
		Facts:       facts,
		ObservedAt:  now,
		Fingerprint: pluginsdk.TriggerFingerprint(w.trigger, o.key()),
		Detail:      o.detail,
	}
}

func shortKind(k pluginsdk.TargetKind) string {
	return strings.TrimPrefix(string(k), "kubernetes/")
}

// admits reports whether the watch may report on an object: in a namespace
// actions may touch, matching actions.selector and the trigger's selector.
func (w *TriggerWatch) admits(namespace string, objLabels map[string]string) bool {
	if namespace != "" && !w.config.inScope(namespace) {
		return false
	}
	set := labels.Set(objLabels)
	return w.scope.Matches(set) && w.selector.Matches(set)
}

// listCheck makes a resourceCheck from a typed list call.
func listCheck[T any](what string, cluster bool, list func(ctx context.Context, ns string, opts metav1.ListOptions) (T, error)) resourceCheck {
	return resourceCheck{what: what, cluster: cluster, list: func(ctx context.Context, ns string) error {
		_, err := list(ctx, ns, metav1.ListOptions{Limit: 1})
		return err
	}}
}

// workloads are the listers workload_unhealthy reads.
type workloads struct {
	pods         []cache.Indexer
	replicaSets  []cache.Indexer
	jobs         []cache.Indexer
	deployments  []cache.Indexer
	statefulSets []cache.Indexer
	daemonSets   []cache.Indexer
	cronJobs     []cache.Indexer
}

func each[T any](indexers []cache.Indexer, fn func(T)) {
	for _, idx := range indexers {
		for _, obj := range idx.List() {
			if t, ok := obj.(T); ok {
				fn(t)
			}
		}
	}
}

func cached[T any](indexers []cache.Indexer, ns, name string) (T, bool) {
	var zero T
	for _, idx := range indexers {
		obj, ok, err := idx.GetByKey(ns + "/" + name)
		if err == nil && ok {
			if t, ok := obj.(T); ok {
				return t, true
			}
		}
	}
	return zero, false
}

// ownerChain resolves a pod to the workloads above it, outermost first,
// through ReplicaSet to Deployment and Job to CronJob. Each link is the
// object's controller reference, which the API server keeps consistent, so
// a pod cannot claim a workload it does not belong to.
func (l *workloads) ownerChain(p *corev1.Pod) []pluginsdk.TriggerTarget {
	ref := metav1.GetControllerOf(p)
	if ref == nil {
		return nil
	}
	ns := p.Namespace
	switch ref.Kind {
	case "ReplicaSet":
		if rs, ok := cached[*appsv1.ReplicaSet](l.replicaSets, ns, ref.Name); ok {
			if up := metav1.GetControllerOf(rs); up != nil && up.Kind == "Deployment" {
				return []pluginsdk.TriggerTarget{{Kind: kindDeployment, Namespace: ns, Name: up.Name}}
			}
		}
		return nil
	case "StatefulSet":
		return []pluginsdk.TriggerTarget{{Kind: kindStatefulSet, Namespace: ns, Name: ref.Name}}
	case "DaemonSet":
		return []pluginsdk.TriggerTarget{{Kind: kindDaemonSet, Namespace: ns, Name: ref.Name}}
	case "Job":
		chain := []pluginsdk.TriggerTarget{{Kind: kindJob, Namespace: ns, Name: ref.Name}}
		if job, ok := cached[*batchv1.Job](l.jobs, ns, ref.Name); ok {
			if up := metav1.GetControllerOf(job); up != nil && up.Kind == "CronJob" {
				chain = append([]pluginsdk.TriggerTarget{{Kind: kindCronJob, Namespace: ns, Name: up.Name}}, chain...)
			}
		}
		return chain
	}
	return nil
}

// labelsOf returns a workload's labels from the cache, or false when the
// cache does not hold it, which leaves it unreported rather than reported
// against a selector it was never checked under.
func (l *workloads) labelsOf(t pluginsdk.TriggerTarget) (map[string]string, bool) {
	var obj metav1.Object
	var ok bool
	switch t.Kind {
	case kindDeployment:
		obj, ok = cached[*appsv1.Deployment](l.deployments, t.Namespace, t.Name)
	case kindStatefulSet:
		obj, ok = cached[*appsv1.StatefulSet](l.statefulSets, t.Namespace, t.Name)
	case kindDaemonSet:
		obj, ok = cached[*appsv1.DaemonSet](l.daemonSets, t.Namespace, t.Name)
	case kindJob:
		obj, ok = cached[*batchv1.Job](l.jobs, t.Namespace, t.Name)
	case kindCronJob:
		obj, ok = cached[*batchv1.CronJob](l.cronJobs, t.Namespace, t.Name)
	}
	if !ok {
		return nil, false
	}
	return obj.GetLabels(), true
}

func (w *TriggerWatch) setupWorkloads(params map[string]any) error {
	kinds := listParam(params, "kinds", []string{"deployment", "statefulset", "daemonset", "job", "cronjob", "pod"})
	conditions := listParam(params, "conditions", workloadConditions)
	w.factories = w.namespacedFactories()
	l := &workloads{}
	for _, f := range w.factories {
		// Pods, ReplicaSets and Jobs are always read, to find each pod's
		// workload; the workloads themselves only for the kinds reported.
		l.pods = append(l.pods, f.Core().V1().Pods().Informer().GetIndexer())
		l.replicaSets = append(l.replicaSets, f.Apps().V1().ReplicaSets().Informer().GetIndexer())
		l.jobs = append(l.jobs, f.Batch().V1().Jobs().Informer().GetIndexer())
		if slices.Contains(kinds, "deployment") {
			l.deployments = append(l.deployments, f.Apps().V1().Deployments().Informer().GetIndexer())
		}
		if slices.Contains(kinds, "statefulset") {
			l.statefulSets = append(l.statefulSets, f.Apps().V1().StatefulSets().Informer().GetIndexer())
		}
		if slices.Contains(kinds, "daemonset") {
			l.daemonSets = append(l.daemonSets, f.Apps().V1().DaemonSets().Informer().GetIndexer())
		}
		if slices.Contains(kinds, "cronjob") {
			l.cronJobs = append(l.cronJobs, f.Batch().V1().CronJobs().Informer().GetIndexer())
		}
	}
	c := w.client
	w.need = []resourceCheck{
		listCheck("pods", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.CoreV1().Pods(ns).List(ctx, o)
		}),
		listCheck("replicasets", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.AppsV1().ReplicaSets(ns).List(ctx, o)
		}),
		listCheck("jobs", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.BatchV1().Jobs(ns).List(ctx, o)
		}),
	}
	if slices.Contains(kinds, "deployment") {
		w.need = append(w.need, listCheck("deployments", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.AppsV1().Deployments(ns).List(ctx, o)
		}))
	}
	if slices.Contains(kinds, "statefulset") {
		w.need = append(w.need, listCheck("statefulsets", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.AppsV1().StatefulSets(ns).List(ctx, o)
		}))
	}
	if slices.Contains(kinds, "daemonset") {
		w.need = append(w.need, listCheck("daemonsets", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.AppsV1().DaemonSets(ns).List(ctx, o)
		}))
	}
	if slices.Contains(kinds, "cronjob") {
		w.need = append(w.need, listCheck("cronjobs", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.BatchV1().CronJobs(ns).List(ctx, o)
		}))
	}
	oomWindow := max(time.Hour, 2*w.forDur)
	w.observe = func(now time.Time) []observation {
		return w.observeWorkloads(l, kinds, conditions, oomWindow, now)
	}
	return nil
}

// podFinding is one condition found on one pod.
type podFinding struct {
	condition, reason string
	restarts          int64
}

// crashWindow is longer than the kubelet's longest restart back-off, five
// minutes, so a crash loop is never out of it between two restarts.
const crashWindow = 10 * time.Minute

func recentlyTerminated(cs corev1.ContainerStatus, now time.Time) bool {
	t := cs.LastTerminationState.Terminated
	return t != nil && now.Sub(t.FinishedAt.Time) < crashWindow
}

var (
	imagePullReasons   = []string{"ImagePullBackOff", "ErrImagePull", "InvalidImageName"}
	configErrorReasons = []string{"CreateContainerConfigError", "CreateContainerError"}
)

// podFindings is what is wrong with one pod now. crash_looping is a
// CrashLoopBackOff, or three or more restarts with the container not ready
// or last stopped within crashWindow: restarts alone only ever grow, so a
// pod that recovered last week would otherwise hold forever, and the second
// test keeps a container that is up for a moment between crashes from
// breaking the condition's continuity. oom_killed holds while the last termination was an
// OOM kill that finished within window, for the same reason.
func podFindings(p *corev1.Pod, oomWindow time.Duration, now time.Time) []podFinding {
	var out []podFinding
	statuses := append(slices.Clone(p.Status.InitContainerStatuses), p.Status.ContainerStatuses...)
	for _, cs := range statuses {
		restarts := int64(cs.RestartCount)
		waiting := ""
		if cs.State.Waiting != nil {
			waiting = cs.State.Waiting.Reason
		}
		switch {
		case waiting == "CrashLoopBackOff":
			out = append(out, podFinding{"crash_looping", waiting, restarts})
		case restarts >= 3 && (!cs.Ready || recentlyTerminated(cs, now)):
			reason := "Restarting"
			if t := cs.LastTerminationState.Terminated; t != nil && t.Reason != "" {
				reason = t.Reason
			}
			out = append(out, podFinding{"crash_looping", reason, restarts})
		}
		if slices.Contains(imagePullReasons, waiting) {
			out = append(out, podFinding{"image_pull_failing", waiting, restarts})
		}
		if slices.Contains(configErrorReasons, waiting) {
			out = append(out, podFinding{"config_error", waiting, restarts})
		}
		for _, t := range []*corev1.ContainerStateTerminated{cs.State.Terminated, cs.LastTerminationState.Terminated} {
			if t != nil && t.Reason == "OOMKilled" && now.Sub(t.FinishedAt.Time) < oomWindow {
				out = append(out, podFinding{"oom_killed", "OOMKilled", restarts})
				break
			}
		}
	}
	for _, c := range p.Status.Conditions {
		switch {
		case c.Type == corev1.PodReady && c.Status != corev1.ConditionTrue && p.Status.Phase == corev1.PodRunning:
			reason := c.Reason
			if reason == "" {
				reason = "NotReady"
			}
			out = append(out, podFinding{"not_ready", reason, 0})
		case c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable:
			out = append(out, podFinding{"unschedulable", c.Reason, 0})
		}
	}
	return out
}

func (w *TriggerWatch) observeWorkloads(l *workloads, kinds, conditions []string, oomWindow time.Duration, now time.Time) []observation {
	found := map[string]*observation{}
	var order []string
	add := func(o observation) {
		key := o.key()
		if have, ok := found[key]; ok {
			have.count += o.count
			have.restartCount = max(have.restartCount, o.restartCount)
			return
		}
		found[key] = &o
		order = append(order, key)
	}
	wanted := func(c string) bool { return slices.Contains(conditions, c) }

	// Pods, reported against the outermost workload of a kind asked for,
	// or as themselves when "pod" is asked for and no workload is.
	var pods []*corev1.Pod
	each(l.pods, func(p *corev1.Pod) { pods = append(pods, p) })
	sort.Slice(pods, func(i, j int) bool { return pods[i].Namespace+"/"+pods[i].Name < pods[j].Namespace+"/"+pods[j].Name })
	for _, p := range pods {
		if p.DeletionTimestamp != nil {
			continue
		}
		findings := podFindings(p, oomWindow, now)
		if len(findings) == 0 {
			continue
		}
		chain := l.ownerChain(p)
		workload := ""
		if len(chain) > 0 {
			workload = shortKind(chain[0].Kind) + "/" + chain[0].Name
		}
		var target *pluginsdk.TriggerTarget
		var targetLabels map[string]string
		for i := range chain {
			if slices.Contains(kinds, shortKind(chain[i].Kind)) {
				objLabels, ok := l.labelsOf(chain[i])
				if !ok {
					break
				}
				target, targetLabels = &chain[i], objLabels
				break
			}
		}
		if target == nil && slices.Contains(kinds, "pod") {
			target = &pluginsdk.TriggerTarget{Kind: kindPod, Namespace: p.Namespace, Name: p.Name}
			targetLabels = p.Labels
		}
		if target == nil || !w.admits(p.Namespace, targetLabels) {
			continue
		}
		seen := map[string]bool{}
		for _, f := range findings {
			if !wanted(f.condition) || seen[f.condition] {
				continue
			}
			seen[f.condition] = true
			add(observation{target: *target, condition: f.condition, reason: f.reason, workload: workload, count: 1, countFact: "pods_affected", restartCount: f.restarts})
		}
	}

	workloadObs := func(t pluginsdk.TriggerTarget, objLabels map[string]string, condition, reason string, count int64) {
		if wanted(condition) && w.admits(t.Namespace, objLabels) {
			add(observation{target: t, condition: condition, reason: reason, workload: shortKind(t.Kind) + "/" + t.Name, count: count, countFact: "pods_affected"})
		}
	}
	each(l.deployments, func(d *appsv1.Deployment) {
		t := pluginsdk.TriggerTarget{Kind: kindDeployment, Namespace: d.Namespace, Name: d.Name}
		desired := replicasOf(d.Spec.Replicas)
		if unavailable := max(d.Status.UnavailableReplicas, desired-d.Status.AvailableReplicas); desired > 0 && unavailable > 0 {
			workloadObs(t, d.Labels, "replicas_unavailable", "MinimumReplicasUnavailable", int64(unavailable))
		}
		for _, c := range d.Status.Conditions {
			if c.Type == appsv1.DeploymentProgressing && c.Status == corev1.ConditionFalse && c.Reason == "ProgressDeadlineExceeded" {
				workloadObs(t, d.Labels, "rollout_stuck", c.Reason, 0)
			}
		}
	})
	each(l.statefulSets, func(s *appsv1.StatefulSet) {
		desired := replicasOf(s.Spec.Replicas)
		if unavailable := desired - s.Status.AvailableReplicas; desired > 0 && unavailable > 0 {
			workloadObs(pluginsdk.TriggerTarget{Kind: kindStatefulSet, Namespace: s.Namespace, Name: s.Name}, s.Labels, "replicas_unavailable", "ReplicasUnavailable", int64(unavailable))
		}
	})
	each(l.daemonSets, func(d *appsv1.DaemonSet) {
		if d.Status.NumberUnavailable > 0 {
			workloadObs(pluginsdk.TriggerTarget{Kind: kindDaemonSet, Namespace: d.Namespace, Name: d.Name}, d.Labels, "replicas_unavailable", "NumberUnavailable", int64(d.Status.NumberUnavailable))
		}
	})

	if wanted("job_failed") {
		// A CronJob is failing when its newest Job failed; an older failed
		// Job that a newer one has since replaced does not count.
		latest := map[string]*batchv1.Job{}
		each(l.jobs, func(j *batchv1.Job) {
			if up := metav1.GetControllerOf(j); up != nil && up.Kind == "CronJob" {
				k := j.Namespace + "/" + up.Name
				if cur := latest[k]; cur == nil || j.CreationTimestamp.After(cur.CreationTimestamp.Time) {
					latest[k] = j
				}
			}
		})
		each(l.jobs, func(j *batchv1.Job) {
			failed := slices.ContainsFunc(j.Status.Conditions, func(c batchv1.JobCondition) bool {
				return c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue
			})
			if !failed {
				return
			}
			reason := "Failed"
			for _, c := range j.Status.Conditions {
				if c.Type == batchv1.JobFailed && c.Reason != "" {
					reason = c.Reason
				}
			}
			if up := metav1.GetControllerOf(j); up != nil && up.Kind == "CronJob" && slices.Contains(kinds, "cronjob") {
				if latest[j.Namespace+"/"+up.Name] != j {
					return
				}
				t := pluginsdk.TriggerTarget{Kind: kindCronJob, Namespace: j.Namespace, Name: up.Name}
				if objLabels, ok := l.labelsOf(t); ok {
					workloadObs(t, objLabels, "job_failed", reason, 0)
				}
				return
			}
			if slices.Contains(kinds, "job") {
				workloadObs(pluginsdk.TriggerTarget{Kind: kindJob, Namespace: j.Namespace, Name: j.Name}, j.Labels, "job_failed", reason, 0)
			}
		})
	}

	out := make([]observation, 0, len(order))
	for _, k := range order {
		out = append(out, *found[k])
	}
	return out
}

func (w *TriggerWatch) setupNodes(params map[string]any) error {
	conditions := listParam(params, "conditions", nodeConditions)
	f := informers.NewSharedInformerFactoryWithOptions(w.client, informerResync, informers.WithTransform(stripManagedFields))
	nodes := f.Core().V1().Nodes().Informer().GetIndexer()
	w.factories = []informers.SharedInformerFactory{f}
	c := w.client
	w.need = []resourceCheck{listCheck("nodes", true, func(ctx context.Context, _ string, o metav1.ListOptions) (any, error) {
		return c.CoreV1().Nodes().List(ctx, o)
	})}
	pressure := map[corev1.NodeConditionType]string{
		corev1.NodeMemoryPressure:     "memory_pressure",
		corev1.NodeDiskPressure:       "disk_pressure",
		corev1.NodePIDPressure:        "pid_pressure",
		corev1.NodeNetworkUnavailable: "network_unavailable",
	}
	w.observe = func(time.Time) []observation {
		var out []observation
		each([]cache.Indexer{nodes}, func(n *corev1.Node) {
			if !w.admits("", n.Labels) {
				return
			}
			t := pluginsdk.TriggerTarget{Kind: kindNode, Name: n.Name}
			add := func(condition, reason string) {
				if slices.Contains(conditions, condition) {
					out = append(out, observation{target: t, condition: condition, reason: reason})
				}
			}
			ready := false
			for _, c := range n.Status.Conditions {
				if c.Type == corev1.NodeReady {
					ready = c.Status == corev1.ConditionTrue
					if !ready {
						add("not_ready", firstNonEmpty(c.Reason, "NotReady"))
					}
				}
				if name, ok := pressure[c.Type]; ok && c.Status == corev1.ConditionTrue {
					add(name, firstNonEmpty(c.Reason, string(c.Type)))
				}
			}
			if !slices.ContainsFunc(n.Status.Conditions, func(c corev1.NodeCondition) bool { return c.Type == corev1.NodeReady }) {
				add("not_ready", "NoReadyCondition")
			}
			if n.Spec.Unschedulable {
				add("unschedulable_for", "Cordoned")
			}
		})
		return out
	}
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (w *TriggerWatch) setupEvents(params map[string]any) error {
	reasons := listParam(params, "reasons", warningReasonNames())
	window := time.Duration(intParam(params, "window_minutes", 10)) * time.Minute
	minCount := intParam(params, "min_count", 3)
	// An event fires when it is seen, not after holding: the window is
	// already the duration.
	w.forDur = 0
	w.factories = w.namespacedFactories()
	var indexers []cache.Indexer
	for _, f := range w.factories {
		indexers = append(indexers, f.Core().V1().Events().Informer().GetIndexer())
	}
	c := w.client
	w.need = []resourceCheck{listCheck("events", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
		return c.CoreV1().Events(ns).List(ctx, o)
	})}
	kinds := map[string]pluginsdk.TargetKind{
		"Pod": kindPod, "Deployment": kindDeployment, "StatefulSet": kindStatefulSet, "DaemonSet": kindDaemonSet,
		"ReplicaSet": kindReplicaSet, "Job": kindJob, "CronJob": kindCronJob, "Node": kindNode,
		"PersistentVolumeClaim": kindPVC, "HorizontalPodAutoscaler": kindAutoscaler,
	}
	w.observe = func(now time.Time) []observation {
		type tally struct {
			obs    observation
			latest time.Time
		}
		tallies := map[string]*tally{}
		var order []string
		each(indexers, func(ev *corev1.Event) {
			if ev.Type != corev1.EventTypeWarning || !slices.Contains(reasons, ev.Reason) {
				return
			}
			last := eventLastSeen(ev)
			if now.Sub(last) > window {
				return
			}
			kind, ok := kinds[ev.InvolvedObject.Kind]
			if !ok {
				return
			}
			ns := ev.InvolvedObject.Namespace
			if kind == kindNode {
				ns = ""
			}
			// Events carry no labels of the object they are about, so only
			// the namespace scope applies, and actions.selector, when set,
			// is honoured by reporting nothing rather than guessing.
			if (ns != "" && !w.config.inScope(ns)) || !w.scope.Empty() || !w.selector.Empty() {
				return
			}
			o := observation{target: pluginsdk.TriggerTarget{Kind: kind, Namespace: ns, Name: ev.InvolvedObject.Name},
				condition: warningReasons[ev.Reason], reason: ev.Reason, countFact: "count"}
			key := o.key()
			t := tallies[key]
			if t == nil {
				t = &tally{obs: o}
				tallies[key] = t
				order = append(order, key)
			}
			n := int64(max(ev.Count, 1))
			if ev.Series != nil {
				n = max(n, int64(ev.Series.Count))
			}
			t.obs.count += n
			if last.After(t.latest) {
				t.latest, t.obs.detail = last, stripControl(ev.Message)
			}
		})
		var out []observation
		for _, k := range order {
			if t := tallies[k]; t.obs.count >= minCount {
				out = append(out, t.obs)
			}
		}
		return out
	}
	return nil
}
