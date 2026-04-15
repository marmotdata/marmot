package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/marmotdata/marmot/internal/core/runs"
)

var runGVR = schema.GroupVersionResource{
	Group:    "runs.marmotdata.io",
	Version:  "v1alpha1",
	Resource: "runs",
}

// Syncer watches Run CRDs in Kubernetes and syncs them into ingestion_schedules.
type Syncer struct {
	client      dynamic.Interface
	scheduleSvc *runs.ScheduleService
	namespace   string
	stopCh      chan struct{}
}

// NewSyncer creates a new Run CRD syncer.
func NewSyncer(scheduleSvc *runs.ScheduleService, namespace string) (*Syncer, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("getting in-cluster config: %w", err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &Syncer{
		client:      client,
		scheduleSvc: scheduleSvc,
		namespace:   namespace,
		stopCh:      make(chan struct{}),
	}, nil
}

// Start begins watching Run CRDs and syncing them to schedules.
func (s *Syncer) Start(ctx context.Context) error {
	var factory dynamicinformer.DynamicSharedInformerFactory
	if s.namespace != "" {
		factory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(s.client, 30*time.Second, s.namespace, nil)
	} else {
		factory = dynamicinformer.NewDynamicSharedInformerFactory(s.client, 30*time.Second)
	}

	informer := factory.ForResource(runGVR).Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			if err := s.syncRun(ctx, u); err != nil {
				log.Error().Err(err).Str("name", u.GetName()).Msg("Failed to sync Run CRD on add")
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			u, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			if err := s.syncRun(ctx, u); err != nil {
				log.Error().Err(err).Str("name", u.GetName()).Msg("Failed to sync Run CRD on update")
			}
		},
		DeleteFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				u, ok = tombstone.Obj.(*unstructured.Unstructured)
				if !ok {
					return
				}
			}
			if err := s.deleteSchedule(ctx, u); err != nil {
				log.Error().Err(err).Str("name", u.GetName()).Msg("Failed to delete schedule for Run CRD")
			}
		},
	})

	go informer.Run(s.stopCh)

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("failed to sync Run CRD informer cache")
	}

	log.Info().Str("namespace", s.namespace).Msg("Run CRD syncer started")
	return nil
}

// Stop halts the syncer.
func (s *Syncer) Stop() {
	close(s.stopCh)
}

// TriggerRun patches the Run CRD with the trigger annotation.
func (s *Syncer) TriggerRun(ctx context.Context, name string) error {
	runObj, err := s.findRunByPipelineName(ctx, name)
	if err != nil {
		return fmt.Errorf("finding Run CRD for pipeline %q: %w", name, err)
	}

	patch := []byte(`{"metadata":{"annotations":{"runs.marmotdata.io/trigger":"true"}}}`)
	_, err = s.client.Resource(runGVR).Namespace(runObj.GetNamespace()).Patch(
		ctx,
		runObj.GetName(),
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("patching Run CRD %q with trigger annotation: %w", runObj.GetName(), err)
	}

	log.Info().Str("run", runObj.GetName()).Str("pipeline", name).Msg("Triggered operator-managed run")
	return nil
}

func (s *Syncer) findRunByPipelineName(ctx context.Context, pipelineName string) (*unstructured.Unstructured, error) {
	var list *unstructured.UnstructuredList
	var err error

	if s.namespace != "" {
		list, err = s.client.Resource(runGVR).Namespace(s.namespace).List(ctx, metav1.ListOptions{})
	} else {
		list, err = s.client.Resource(runGVR).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("listing Run CRDs: %w", err)
	}

	for i := range list.Items {
		item := &list.Items[i]
		specName, _, _ := unstructured.NestedString(item.Object, "spec", "name")
		if specName == pipelineName {
			return item, nil
		}
	}

	return nil, fmt.Errorf("Run CRD with spec.name=%q not found", pipelineName)
}

func (s *Syncer) syncRun(ctx context.Context, u *unstructured.Unstructured) error {
	specName, _, _ := unstructured.NestedString(u.Object, "spec", "name")
	if specName == "" {
		specName = u.GetName()
	}

	schedule, _, _ := unstructured.NestedString(u.Object, "spec", "schedule")

	// Extract plugin ID and config from the first run entry
	pluginID := ""
	config := map[string]interface{}{}
	runsSlice, found, _ := unstructured.NestedSlice(u.Object, "spec", "runs")
	if found && len(runsSlice) > 0 {
		if entry, ok := runsSlice[0].(map[string]interface{}); ok {
			for k, v := range entry {
				pluginID = k
				if cfg, ok := v.(map[string]interface{}); ok {
					config = cfg
				}
				break
			}
		}
	}

	// If we couldn't extract from the slice (may be stored as raw JSON), try raw
	if pluginID == "" {
		rawRuns, found, _ := unstructured.NestedFieldNoCopy(u.Object, "spec", "runs")
		if found {
			if rawSlice, ok := rawRuns.([]interface{}); ok && len(rawSlice) > 0 {
				raw, _ := json.Marshal(rawSlice[0])
				var entry map[string]interface{}
				if json.Unmarshal(raw, &entry) == nil {
					for k, v := range entry {
						pluginID = k
						if cfg, ok := v.(map[string]interface{}); ok {
							config = cfg
						}
						break
					}
				}
			}
		}
	}

	managedBy := "operator"
	_, err := s.scheduleSvc.SyncSchedule(ctx, specName, pluginID, config, schedule, managedBy)
	if err != nil {
		return fmt.Errorf("syncing schedule for Run %q: %w", specName, err)
	}

	log.Debug().Str("run", u.GetName()).Str("pipeline", specName).Str("plugin", pluginID).Msg("Synced Run CRD to schedule")
	return nil
}

func (s *Syncer) deleteSchedule(ctx context.Context, u *unstructured.Unstructured) error {
	specName, _, _ := unstructured.NestedString(u.Object, "spec", "name")
	if specName == "" {
		specName = u.GetName()
	}

	schedule, err := s.scheduleSvc.GetScheduleByName(ctx, specName)
	if err != nil {
		if errors.Is(err, runs.ErrScheduleNotFound) {
			return nil // Already gone
		}
		return err
	}

	// Only delete if it's operator-managed
	if schedule.ManagedBy != nil && *schedule.ManagedBy == "operator" {
		if err := s.scheduleSvc.DeleteSchedule(ctx, schedule.ID); err != nil {
			return fmt.Errorf("deleting schedule %q: %w", specName, err)
		}
		log.Info().Str("pipeline", specName).Msg("Deleted operator-managed schedule")
	}

	return nil
}
