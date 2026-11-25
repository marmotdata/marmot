package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// custom_condition runs a CEL expression the runner's operator wrote over
// each watched object. CEL is chosen because it cannot loop without bound,
// call out, or change anything: it is the language Kubernetes itself uses
// for admission and validation rules. Still, the expression is bounded
// here twice: a cost limit, which stops runaway list macros, and a
// wall-clock timeout per object. The expression decides only whether the
// condition holds; the event's facts stay the fixed set, so it cannot put
// free text into what reaches Marmot.

const (
	// maxCELLength bounds the expression.
	maxCELLength = 2048
	// celCostLimit bounds one evaluation, in CEL's cost units, about one
	// per field read or comparison: ample for any condition over one
	// object, far short of what a quadratic walk over a big list costs.
	celCostLimit = 100_000
	// celTimeout bounds one evaluation's wall clock.
	celTimeout = 50 * time.Millisecond
)

// customKinds are the kinds custom_condition can watch.
var customKinds = []string{"pod", "deployment", "statefulset", "daemonset", "job", "cronjob", "node", "persistentvolumeclaim", "horizontalpodautoscaler"}

var celEnv = sync.OnceValues(func() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("object", cel.DynType),
		// object.status.?availableReplicas.orValue(0), as Kubernetes CEL
		// rules are written.
		cel.OptionalTypes(),
		ext.Strings(), ext.Lists(), ext.Sets(),
	)
})

// CompileCondition compiles a custom_condition expression, refusing one
// that does not compile or whose result is not a bool. An expression whose
// type CEL cannot know until it runs, such as object.spec.paused, is refused
// too, so a result that is not a bool can never be read as true: compare
// it, as in object.spec.paused == true.
func CompileCondition(expr string) (cel.Program, error) {
	if len(expr) > maxCELLength {
		return nil, invalid("when is %d bytes, above the %d allowed", len(expr), maxCELLength)
	}
	env, err := celEnv()
	if err != nil {
		return nil, fmt.Errorf("building the CEL environment: %w", err)
	}
	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, invalid("when does not compile: %v", issues.Err())
	}
	if !ast.OutputType().IsExactType(cel.BoolType) {
		return nil, invalid("when must return a bool, and returns %s", ast.OutputType())
	}
	prg, err := env.Program(ast, cel.CostLimit(celCostLimit), cel.InterruptCheckFrequency(100))
	if err != nil {
		return nil, invalid("when cannot run: %v", err)
	}
	return prg, nil
}

// evalCondition runs the expression over one object. Any error, a cost or
// time overrun included, is false: a condition that cannot be evaluated is
// not known to hold.
func evalCondition(prg cel.Program, object map[string]any) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), celTimeout)
	defer cancel()
	out, _, err := prg.ContextEval(ctx, map[string]any{"object": object})
	if err != nil {
		return false, err
	}
	b, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("returned %v, not a bool", out.Type())
	}
	return b, nil
}

func (w *TriggerWatch) setupCustom(params map[string]any) error {
	kind, _ := params["kind"].(string)
	condition, _ := params["name"].(string)
	expr, _ := params["when"].(string)
	prg, err := CompileCondition(expr)
	if err != nil {
		return err
	}

	targetKind := pluginsdk.TargetKind("kubernetes/" + kind)
	apiVersion, apiKind := customAPI(kind)
	c := w.client
	var indexers []cache.Indexer
	l := &workloads{}
	if kind == "node" {
		f := informers.NewSharedInformerFactoryWithOptions(c, informerResync, informers.WithTransform(stripManagedFields))
		w.factories = []informers.SharedInformerFactory{f}
		indexers = []cache.Indexer{f.Core().V1().Nodes().Informer().GetIndexer()}
		w.need = []resourceCheck{listCheck("nodes", true, func(ctx context.Context, _ string, o metav1.ListOptions) (any, error) {
			return c.CoreV1().Nodes().List(ctx, o)
		})}
	} else {
		w.factories = w.namespacedFactories()
		for _, f := range w.factories {
			var inf cache.SharedIndexInformer
			switch kind {
			case "pod":
				inf = f.Core().V1().Pods().Informer()
				// A pod's workload is found as workload_unhealthy finds it.
				l.replicaSets = append(l.replicaSets, f.Apps().V1().ReplicaSets().Informer().GetIndexer())
				l.jobs = append(l.jobs, f.Batch().V1().Jobs().Informer().GetIndexer())
			case "deployment":
				inf = f.Apps().V1().Deployments().Informer()
			case "statefulset":
				inf = f.Apps().V1().StatefulSets().Informer()
			case "daemonset":
				inf = f.Apps().V1().DaemonSets().Informer()
			case "job":
				inf = f.Batch().V1().Jobs().Informer()
			case "cronjob":
				inf = f.Batch().V1().CronJobs().Informer()
			case "persistentvolumeclaim":
				inf = f.Core().V1().PersistentVolumeClaims().Informer()
			case "horizontalpodautoscaler":
				inf = f.Autoscaling().V2().HorizontalPodAutoscalers().Informer()
			default:
				return invalid("custom_condition cannot watch kind %q", kind)
			}
			indexers = append(indexers, inf.GetIndexer())
		}
		w.need = []resourceCheck{customCheck(w, kind)}
		if kind == "pod" {
			w.need = append(w.need,
				listCheck("replicasets", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
					return c.AppsV1().ReplicaSets(ns).List(ctx, o)
				}),
				listCheck("jobs", false, func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
					return c.BatchV1().Jobs(ns).List(ctx, o)
				}))
		}
	}

	// Errors are logged once per object until it evaluates cleanly again,
	// so an expression that fails on every object does not flood the log.
	failing := map[string]bool{}
	w.observe = func(time.Time) []observation {
		var out []observation
		for _, idx := range indexers {
			for _, item := range idx.List() {
				obj, ok := item.(runtime.Object)
				if !ok {
					continue
				}
				m, err := metav1Accessor(obj)
				if err != nil || !w.admits(m.GetNamespace(), m.GetLabels()) {
					continue
				}
				id := m.GetNamespace() + "/" + m.GetName()
				u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
				if err != nil {
					continue
				}
				// Informers drop apiVersion and kind; an expression may read
				// them, as it could in Kubernetes' own CEL.
				u["apiVersion"], u["kind"] = apiVersion, apiKind
				holds, err := evalCondition(prg, u)
				if err != nil {
					if !failing[id] {
						log.Warn().Err(err).Str("trigger", w.trigger).Str("object", kind+"/"+id).Msg("The custom condition could not be evaluated; it counts as false")
						failing[id] = true
					}
					continue
				}
				delete(failing, id)
				if !holds {
					continue
				}
				o := observation{target: pluginsdk.TriggerTarget{Kind: targetKind, Namespace: m.GetNamespace(), Name: m.GetName()}, condition: condition}
				if p, ok := obj.(*corev1.Pod); ok {
					if chain := l.ownerChain(p); len(chain) > 0 {
						o.workload = shortKind(chain[0].Kind) + "/" + chain[0].Name
					}
				} else if kind != "node" {
					o.workload = kind + "/" + m.GetName()
				}
				out = append(out, o)
			}
		}
		if len(failing) > maxTracked {
			clear(failing)
		}
		return out
	}
	return nil
}

func metav1Accessor(obj runtime.Object) (metav1.Object, error) {
	m, ok := obj.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("not an object")
	}
	return m, nil
}

func customAPI(kind string) (apiVersion, apiKind string) {
	switch kind {
	case "pod":
		return "v1", "Pod"
	case "node":
		return "v1", "Node"
	case "persistentvolumeclaim":
		return "v1", "PersistentVolumeClaim"
	case "deployment":
		return "apps/v1", "Deployment"
	case "statefulset":
		return "apps/v1", "StatefulSet"
	case "daemonset":
		return "apps/v1", "DaemonSet"
	case "job":
		return "batch/v1", "Job"
	case "cronjob":
		return "batch/v1", "CronJob"
	case "horizontalpodautoscaler":
		return "autoscaling/v2", "HorizontalPodAutoscaler"
	}
	return "", ""
}

func customCheck(w *TriggerWatch, kind string) resourceCheck {
	c := w.client
	l := func(what string, fn func(ctx context.Context, ns string, o metav1.ListOptions) (any, error)) resourceCheck {
		return listCheck(what, false, fn)
	}
	switch kind {
	case "pod":
		return l("pods", func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.CoreV1().Pods(ns).List(ctx, o)
		})
	case "deployment":
		return l("deployments", func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.AppsV1().Deployments(ns).List(ctx, o)
		})
	case "statefulset":
		return l("statefulsets", func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.AppsV1().StatefulSets(ns).List(ctx, o)
		})
	case "daemonset":
		return l("daemonsets", func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.AppsV1().DaemonSets(ns).List(ctx, o)
		})
	case "job":
		return l("jobs", func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.BatchV1().Jobs(ns).List(ctx, o)
		})
	case "cronjob":
		return l("cronjobs", func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.BatchV1().CronJobs(ns).List(ctx, o)
		})
	case "persistentvolumeclaim":
		return l("persistentvolumeclaims", func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.CoreV1().PersistentVolumeClaims(ns).List(ctx, o)
		})
	default:
		return l("horizontalpodautoscalers", func(ctx context.Context, ns string, o metav1.ListOptions) (any, error) {
			return c.AutoscalingV2().HorizontalPodAutoscalers(ns).List(ctx, o)
		})
	}
}
