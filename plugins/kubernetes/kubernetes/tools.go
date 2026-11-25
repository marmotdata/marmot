package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ToolsConfig bounds what the plugin's tools may change, on top of the
// RBAC and admission policy its service account runs under. It is the
// runner's own config, so a caller choosing a tool cannot widen it.
type ToolsConfig struct {
	MaxReplicas        int      `json:"max_replicas,omitempty" description:"Highest replica count the scale tool may set, from 1 to 1000" default:"20"`
	Namespaces         []string `json:"namespaces,omitempty" description:"Namespaces tools may change. Empty allows every namespace except kube-system, kube-public and kube-node-lease"`
	Selector           string   `json:"selector,omitempty" description:"Label selector every target must match, nodes included (e.g. marmot.io/tools=enabled)"`
	AllowGitOpsManaged bool     `json:"allow_gitops_managed,omitempty" label:"Allow GitOps-managed objects" description:"Allow tools on objects that Flux or Argo CD manage. Their changes normally belong in Git, and the next reconcile may revert them"`
}

const (
	// defaultMaxReplicas applies when tools.max_replicas is unset.
	defaultMaxReplicas = 20
	// replicasCeiling is the manifest's bound on scale, which is static so
	// its digest is too; tools.max_replicas narrows it per runner.
	replicasCeiling = 1000
)

// systemNamespaces are refused unless tools.namespaces names them, since
// the cluster's own components live there.
var systemNamespaces = []string{"kube-system", "kube-public", "kube-node-lease"}

// Validate checks the tools settings.
func (c *ToolsConfig) Validate() error {
	if c.MaxReplicas < 0 || c.MaxReplicas > replicasCeiling {
		return fmt.Errorf("tools.max_replicas must be between 1 and %d", replicasCeiling)
	}
	for _, ns := range c.Namespaces {
		if errs := validation.IsDNS1123Label(ns); len(errs) > 0 {
			return fmt.Errorf("tools.namespaces: %q is not a namespace name", ns)
		}
	}
	if _, err := labels.Parse(c.Selector); err != nil {
		return fmt.Errorf("invalid tools.selector %q: %w", c.Selector, err)
	}
	return nil
}

func (c *ToolsConfig) maxReplicas() int64 {
	if c.MaxReplicas == 0 {
		return defaultMaxReplicas
	}
	return int64(c.MaxReplicas)
}

// Target kinds, for policy to scope tools by what they change.
const (
	kindDeployment  pluginsdk.TargetKind = "kubernetes/deployment"
	kindStatefulSet pluginsdk.TargetKind = "kubernetes/statefulset"
	kindDaemonSet   pluginsdk.TargetKind = "kubernetes/daemonset"
	kindCronJob     pluginsdk.TargetKind = "kubernetes/cronjob"
	kindNode        pluginsdk.TargetKind = "kubernetes/node"
)

// Tool names.
const (
	toolScale          = "scale"
	toolRolloutRestart = "rollout_restart"
	toolRolloutUndo    = "rollout_undo"
	toolCordon         = "cordon"
	toolUncordon       = "uncordon"
	toolSuspendCronJob = "suspend_cronjob"
	toolResumeCronJob  = "resume_cronjob"
)

// Patterns are Kubernetes' own name rules, so a parameter that passes them
// can only ever name an object, never smuggle a path or selector.
const (
	dns1123Label     = `[a-z0-9]([-a-z0-9]*[a-z0-9])?`
	dns1123Subdomain = `[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*`
)

var (
	namespaceParam = pluginsdk.ToolParam{
		Name: "namespace", Description: "Namespace of the object", Type: pluginsdk.ParamString,
		Required: true, MaxLength: 63, Pattern: dns1123Label,
	}
	nameParam = pluginsdk.ToolParam{
		Name: "name", Description: "Name of the object", Type: pluginsdk.ParamString,
		Required: true, MaxLength: 253, Pattern: dns1123Subdomain,
	}
)

// ToolSpecs is the plugin's tool manifest. Every write changes one
// existing object through a fixed patch: none creates or deletes anything,
// reads a Secret, or reaches into a pod, and there is no generic patch. The
// reads, at the end, return status, logs and events and change nothing.
func ToolSpecs() []pluginsdk.ToolSpec {
	return append(writeSpecs(), readSpecs()...)
}

func writeSpecs() []pluginsdk.ToolSpec {
	workloadKind := func(kinds ...string) pluginsdk.ToolParam {
		return pluginsdk.ToolParam{
			Name: "kind", Description: "Kind of workload", Type: pluginsdk.ParamEnum, Required: true, Enum: kinds,
		}
	}
	return []pluginsdk.ToolSpec{
		{
			Name:        toolScale,
			Version:     1,
			Title:       "Scale a workload",
			Description: "Sets the replica count of a Deployment or StatefulSet. Undone by scaling back to the previous count.",
			Targets:     []pluginsdk.TargetKind{kindDeployment, kindStatefulSet},
			Params: []pluginsdk.ToolParam{
				workloadKind("deployment", "statefulset"), namespaceParam, nameParam,
				{Name: "replicas", Description: "New replica count, capped by the runner's tools.max_replicas", Type: pluginsdk.ParamInt, Required: true, Min: 0, Max: replicasCeiling},
			},
			Reversibility: pluginsdk.ReversibilityReversible,
			// Reversible, but scaling to zero is an outage and scaling
			// up costs money, so it is never the lowest tier.
			Risk:    pluginsdk.RiskCostly,
			Inverse: toolScale,
		},
		{
			Name:          toolRolloutRestart,
			Version:       1,
			Title:         "Restart a workload",
			Description:   "Replaces a workload's pods with a rolling update, as kubectl rollout restart does, by stamping the pod template's restartedAt annotation.",
			Targets:       []pluginsdk.TargetKind{kindDeployment, kindStatefulSet, kindDaemonSet},
			Params:        []pluginsdk.ToolParam{workloadKind("deployment", "statefulset", "daemonset"), namespaceParam, nameParam},
			Reversibility: pluginsdk.ReversibilitySelfHealing,
			Risk:          pluginsdk.RiskBounded,
		},
		{
			Name:        toolRolloutUndo,
			Version:     1,
			Title:       "Roll back a Deployment",
			Description: "Restores a Deployment's pod template from an earlier revision, the one before the current by default, as kubectl rollout undo does. Undone by rolling back to the revision it replaced.",
			Targets:     []pluginsdk.TargetKind{kindDeployment},
			Params: []pluginsdk.ToolParam{
				namespaceParam, nameParam,
				{Name: "to_revision", Description: "Revision to restore. Defaults to the one before the current", Type: pluginsdk.ParamInt, Min: 1, Max: math.MaxInt32},
			},
			Reversibility: pluginsdk.ReversibilityReversible,
			// It changes what code runs.
			Risk:    pluginsdk.RiskCostly,
			Inverse: toolRolloutUndo,
		},
		{
			Name:          toolCordon,
			Version:       1,
			Title:         "Cordon a node",
			Description:   "Marks a node unschedulable so no new pods land on it. Pods already there keep running; nothing is drained.",
			Targets:       []pluginsdk.TargetKind{kindNode},
			Params:        []pluginsdk.ToolParam{{Name: "name", Description: "Name of the node", Type: pluginsdk.ParamString, Required: true, MaxLength: 253, Pattern: dns1123Subdomain}},
			Reversibility: pluginsdk.ReversibilityReversible,
			Risk:          pluginsdk.RiskBounded,
			Inverse:       toolUncordon,
		},
		{
			Name:          toolUncordon,
			Version:       1,
			Title:         "Uncordon a node",
			Description:   "Marks a node schedulable again.",
			Targets:       []pluginsdk.TargetKind{kindNode},
			Params:        []pluginsdk.ToolParam{{Name: "name", Description: "Name of the node", Type: pluginsdk.ParamString, Required: true, MaxLength: 253, Pattern: dns1123Subdomain}},
			Reversibility: pluginsdk.ReversibilityReversible,
			Risk:          pluginsdk.RiskBounded,
			Inverse:       toolCordon,
		},
		{
			Name:          toolSuspendCronJob,
			Version:       1,
			Title:         "Suspend a CronJob",
			Description:   "Stops a CronJob starting new runs. Runs already started carry on.",
			Targets:       []pluginsdk.TargetKind{kindCronJob},
			Params:        []pluginsdk.ToolParam{namespaceParam, nameParam},
			Reversibility: pluginsdk.ReversibilityReversible,
			Risk:          pluginsdk.RiskBounded,
			Inverse:       toolResumeCronJob,
		},
		{
			Name:          toolResumeCronJob,
			Version:       1,
			Title:         "Resume a CronJob",
			Description:   "Lets a suspended CronJob start runs again.",
			Targets:       []pluginsdk.TargetKind{kindCronJob},
			Params:        []pluginsdk.ToolParam{namespaceParam, nameParam},
			Reversibility: pluginsdk.ReversibilityReversible,
			Risk:          pluginsdk.RiskBounded,
			Inverse:       toolSuspendCronJob,
		},
	}
}

// Tools returns the plugin's tool manifest.
func (s *Source) Tools() []pluginsdk.ToolSpec {
	return ToolSpecs()
}

// PlanTool previews a tool with a server-side dry run.
func (s *Source) PlanTool(ctx context.Context, rawConfig pluginsdk.RawConfig, req pluginsdk.ToolRequest) (*pluginsdk.ToolPlan, error) {
	engine, err := s.toolEngine(rawConfig, req.Origin)
	if err != nil {
		return nil, err
	}
	plan, err := engine.Plan(ctx, req)
	return plan, wireError(err)
}

// ExecuteTool runs a tool against the state its plan saw.
func (s *Source) ExecuteTool(ctx context.Context, rawConfig pluginsdk.RawConfig, req pluginsdk.ToolRequest) (*pluginsdk.ToolResult, error) {
	engine, err := s.toolEngine(rawConfig, req.Origin)
	if err != nil {
		return nil, err
	}
	result, err := engine.Execute(ctx, req)
	return result, wireError(err)
}

// wireError gives a refusal the gRPC code that says what to do about it. Errors crossing the plugin boundary otherwise all arrive as Unknown, and Marmot has to tell "plan it again" (FailedPrecondition) and "not allowed here" (PermissionDenied) apart from a fault without parsing messages.
func wireError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, ErrStale), errors.Is(err, ErrLastNode), apierrors.IsConflict(err):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, ErrOutOfScope), errors.Is(err, ErrGitOpsManaged), apierrors.IsForbidden(err):
		return status.Error(codes.PermissionDenied, err.Error())
	case apierrors.IsNotFound(err):
		return status.Error(codes.NotFound, err.Error())
	case apierrors.IsInvalid(err):
		// The cluster's admission policy, or a JSON patch test on the resourceVersion, refused the change
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	return err
}

func (s *Source) toolEngine(rawConfig pluginsdk.RawConfig, origin string) (*ToolEngine, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	client := s.client
	if client == nil {
		if client, err = newToolClient(config, origin); err != nil {
			return nil, fmt.Errorf("creating Kubernetes client: %w", err)
		}
	}
	return NewToolEngine(client, config.Tools)
}

// newToolClient builds a client whose User-Agent names the origin, so the
// API server's audit log ties every request to the run that made it. A
// kubeconfig that impersonates is refused: tools run as the runner's own
// service account and nothing else, which is what the chart's RBAC and
// admission policy are written against.
func newToolClient(config *Config, origin string) (k8s.Interface, error) {
	restConfig, err := buildRestConfig(config)
	if err != nil {
		return nil, err
	}
	imp := restConfig.Impersonate
	if imp.UserName != "" || imp.UID != "" || len(imp.Groups) > 0 || len(imp.Extra) > 0 {
		return nil, fmt.Errorf("the kubeconfig impersonates %q, which tools never do: use the runner's own credentials", imp.UserName)
	}
	restConfig = rest.CopyConfig(restConfig)
	// The SDK has already refused an origin that is not printable ASCII
	// without spaces, so it cannot split the header.
	restConfig.UserAgent = fmt.Sprintf("marmot-plugin-kubernetes/tools (origin %s)", origin)
	return k8s.NewForConfig(restConfig)
}

// inScope reports whether tools may touch the namespace.
func (c *ToolsConfig) inScope(namespace string) bool {
	if len(c.Namespaces) > 0 {
		return slices.Contains(c.Namespaces, namespace)
	}
	return !slices.Contains(systemNamespaces, namespace)
}
