package kubernetes

import (
	"context"
	"fmt"
	"slices"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/labels"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Triggers watch the cluster and report what goes wrong as typed events.
// They only ever read. Which triggers run, and every parameter they take,
// comes from the runner's own local policy, which passes them to Watch:
// Marmot never sends a trigger, a parameter or an expression to the plugin,
// it only receives the events. That is why custom_condition may carry a CEL
// expression at all: it is the cluster operator's own, written into the
// runner's policy, never something Marmot or a task chose.

// Trigger names.
const (
	triggerWorkloadUnhealthy = "workload_unhealthy"
	triggerNodeUnhealthy     = "node_unhealthy"
	triggerWarningEvents     = "warning_events"
	triggerCustomCondition   = "custom_condition"
)

// Further target kinds a trigger can name.
const (
	kindJob           pluginsdk.TargetKind = "kubernetes/job"
	kindReplicaSet    pluginsdk.TargetKind = "kubernetes/replicaset"
	kindPVC           pluginsdk.TargetKind = "kubernetes/persistentvolumeclaim"
	kindAutoscaler    pluginsdk.TargetKind = "kubernetes/horizontalpodautoscaler"
	maxForMinutes                          = 240
	defaultForMinutes                      = 10
)

// workloadConditions are workload_unhealthy's conditions, in the order a
// target's events are emitted.
var workloadConditions = []string{
	"crash_looping", "image_pull_failing", "config_error", "oom_killed", "not_ready",
	"unschedulable", "replicas_unavailable", "rollout_stuck", "job_failed",
}

var nodeConditions = []string{
	"not_ready", "memory_pressure", "disk_pressure", "pid_pressure", "network_unavailable", "unschedulable_for",
}

// warningReasons are the Warning event reasons warning_events can count,
// each with the condition it reports. The list is closed so a policy names
// reasons Kubernetes really emits, not text a workload could make up.
var warningReasons = map[string]string{
	"BackOff":            "back_off",
	"FailedScheduling":   "failed_scheduling",
	"FailedMount":        "failed_mount",
	"FailedAttachVolume": "failed_attach_volume",
	"Evicted":            "evicted",
	"OOMKilling":         "oom_killing",
	"FailedCreate":       "failed_create",
	"Unhealthy":          "unhealthy",
	"NodeNotReady":       "node_not_ready",
}

func warningReasonNames() []string {
	names := make([]string, 0, len(warningReasons))
	for r := range warningReasons {
		names = append(names, r)
	}
	slices.Sort(names)
	return names
}

func warningConditionNames() []string {
	names := make([]string, 0, len(warningReasons))
	for _, c := range warningReasons {
		names = append(names, c)
	}
	slices.Sort(names)
	return names
}

var (
	namespacesTriggerParam = pluginsdk.ActionParam{
		Name: "namespaces", Description: "Namespaces to watch, each within actions.namespaces. Empty watches every namespace actions may see, which across the cluster needs the chart's triggers.clusterWide",
		Type: pluginsdk.ParamStringList, MaxItems: 64, MaxLength: 63, Pattern: dns1123Label,
	}
	selectorTriggerParam = pluginsdk.ActionParam{
		Name: "selector", Description: "Label selector the target must match, on top of actions.selector", Type: pluginsdk.ParamString, MaxLength: 512,
	}
	forMinutesTriggerParam = pluginsdk.ActionParam{
		Name: "for_minutes", Description: "How long a condition must hold, without a break, before it fires. Defaults to 10",
		Type: pluginsdk.ParamInt, Min: 1, Max: maxForMinutes,
	}
)

// TriggerSpecs is the plugin's trigger manifest.
func TriggerSpecs() []pluginsdk.TriggerSpec {
	return []pluginsdk.TriggerSpec{
		{
			Name:        triggerWorkloadUnhealthy,
			Version:     1,
			Title:       "Workload unhealthy",
			Description: "Fires once per workload and condition when a workload or its pods have been unhealthy for for_minutes: crash looping, failing to pull images, misconfigured, OOM killed, not ready, unschedulable, short of replicas, a stuck rollout or a failed job. A pod's condition is reported against the workload that owns it. It clears when the condition does and can fire again.",
			Targets:     []pluginsdk.TargetKind{kindPod, kindDeployment, kindStatefulSet, kindDaemonSet, kindJob, kindCronJob},
			Conditions:  workloadConditions,
			Params: []pluginsdk.ActionParam{
				{Name: "kinds", Description: "Kinds of target to report. Defaults to every kind", Type: pluginsdk.ParamEnumList, MaxItems: 6,
					Enum: []string{"deployment", "statefulset", "daemonset", "job", "cronjob", "pod"}},
				namespacesTriggerParam, selectorTriggerParam,
				{Name: "conditions", Description: "Conditions to report. Defaults to every condition", Type: pluginsdk.ParamEnumList, MaxItems: len(workloadConditions), Enum: workloadConditions},
				forMinutesTriggerParam,
			},
		},
		{
			Name:        triggerNodeUnhealthy,
			Version:     1,
			Title:       "Node unhealthy",
			Description: "Fires once per node and condition when a node has been not ready, under memory, disk or PID pressure, without its network, or cordoned, for for_minutes. Needs the chart's triggers.clusterWide, since nodes belong to no namespace.",
			Targets:     []pluginsdk.TargetKind{kindNode},
			Conditions:  nodeConditions,
			Params: []pluginsdk.ActionParam{
				selectorTriggerParam,
				{Name: "conditions", Description: "Conditions to report. Defaults to every condition", Type: pluginsdk.ParamEnumList, MaxItems: len(nodeConditions), Enum: nodeConditions},
				forMinutesTriggerParam,
			},
		},
		{
			Name:        triggerWarningEvents,
			Version:     1,
			Title:       "Warning events",
			Description: "Fires when an object has gathered at least min_count Warning events of one reason within window_minutes. Events are best effort and can be dropped or merged, so prefer workload_unhealthy and node_unhealthy, which read state; this is for what state does not show, such as failed mounts. The event message travels only as untrusted detail.",
			Targets:     []pluginsdk.TargetKind{kindPod, kindDeployment, kindStatefulSet, kindDaemonSet, kindReplicaSet, kindJob, kindCronJob, kindNode, kindPVC, kindAutoscaler},
			Conditions:  warningConditionNames(),
			Params: []pluginsdk.ActionParam{
				{Name: "reasons", Description: "Event reasons to count. Defaults to every one listed", Type: pluginsdk.ParamEnumList, MaxItems: len(warningReasons), Enum: warningReasonNames()},
				namespacesTriggerParam,
				{Name: "window_minutes", Description: "How far back events count. Defaults to 10", Type: pluginsdk.ParamInt, Min: 1, Max: 60},
				{Name: "min_count", Description: "How many events of one reason on one object fire the trigger. Defaults to 3", Type: pluginsdk.ParamInt, Min: 1, Max: 100},
			},
		},
		{
			Name:        triggerCustomCondition,
			Version:     1,
			Title:       "Custom condition",
			Description: "Fires once per object when a CEL expression the runner's operator wrote has been true of it for for_minutes. The expression sees the object as `object` and must return a bool; it is compiled when the watch starts, and one that does not compile or return a bool stops the watch from starting. An evaluation error counts as false.",
			Targets:     []pluginsdk.TargetKind{kindPod, kindDeployment, kindStatefulSet, kindDaemonSet, kindJob, kindCronJob, kindNode, kindPVC, kindAutoscaler},
			Params: []pluginsdk.ActionParam{
				{Name: "kind", Description: "Kind of object to watch", Type: pluginsdk.ParamEnum, Required: true, Enum: customKinds},
				namespacesTriggerParam, selectorTriggerParam,
				{Name: "when", Description: "CEL expression over object returning bool, such as object.status.?availableReplicas.orValue(0) < object.spec.replicas", Type: pluginsdk.ParamString, Required: true, MaxLength: maxCELLength},
				{Name: "name", Description: "Label for the condition, which events carry as their condition", Type: pluginsdk.ParamString, Required: true, MaxLength: 40, Pattern: `[a-z0-9_]{1,40}`},
				forMinutesTriggerParam,
			},
		},
	}
}

// Triggers returns the plugin's trigger manifest.
func (s *Source) Triggers() []pluginsdk.TriggerSpec {
	return TriggerSpecs()
}

// Watch runs one trigger until ctx ends. Anything wrong with its
// parameters, scope or access fails here, before any event, so a runner
// whose policy asks for something the plugin or the cluster will not allow
// learns so when it starts rather than by silence.
func (s *Source) Watch(ctx context.Context, rawConfig pluginsdk.RawConfig, trigger string, params map[string]any, emit func(pluginsdk.TriggerEvent) error) error {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return fmt.Errorf("unmarshaling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	client := s.client
	if client == nil {
		if client, err = newWatchClient(config); err != nil {
			return fmt.Errorf("creating Kubernetes client: %w", err)
		}
	}
	w, err := NewTriggerWatch(client, config.Actions, trigger, params, emit)
	if err != nil {
		return err
	}
	return w.Run(ctx)
}

// newWatchClient builds a client for triggers, refusing impersonation for
// the same reason actions do: the chart's RBAC is written for the runner's
// own service account.
func newWatchClient(config *Config) (k8s.Interface, error) {
	restConfig, err := buildRestConfig(config)
	if err != nil {
		return nil, err
	}
	imp := restConfig.Impersonate
	if imp.UserName != "" || imp.UID != "" || len(imp.Groups) > 0 || len(imp.Extra) > 0 {
		return nil, fmt.Errorf("the kubeconfig impersonates %q, which triggers never do: use the runner's own credentials", imp.UserName)
	}
	restConfig = rest.CopyConfig(restConfig)
	restConfig.UserAgent = "marmot-plugin-kubernetes/triggers"
	return k8s.NewForConfig(restConfig)
}

// triggerParams is a trigger's parameters with the defaults filled in.
type triggerParams struct {
	namespaces []string
	selector   labels.Selector
	set        func(name string) []string
	forMinutes int64
}

func invalid(format string, args ...any) error {
	return status.Errorf(codes.InvalidArgument, format, args...)
}

func listParam(params map[string]any, name string, all []string) []string {
	l, ok := params[name].([]string)
	if !ok || len(l) == 0 {
		return all
	}
	return l
}

func intParam(params map[string]any, name string, def int64) int64 {
	n, ok := pluginsdk.ActionRequest{Params: params}.IntParam(name)
	if !ok {
		return def
	}
	return n
}

// scopeNamespaces resolves a trigger's namespaces against actions.namespaces:
// each named one must be in scope, and none named means every namespace in
// scope, which is the whole cluster bar the system namespaces when
// actions.namespaces is empty. A nil result means cluster wide.
func scopeNamespaces(config ActionsConfig, named []string) ([]string, error) {
	for _, ns := range named {
		if !config.inScope(ns) {
			return nil, status.Errorf(codes.PermissionDenied, "namespace %q is outside the actions scope: allow it in actions.namespaces", ns)
		}
	}
	if len(named) > 0 {
		return named, nil
	}
	return config.Namespaces, nil
}
