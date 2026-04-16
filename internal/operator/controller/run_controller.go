package controller

import (
	"context"
	"fmt"
	"math"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	runsv1alpha1 "github.com/marmotdata/marmot/internal/operator/api/v1alpha1"
)

const (
	finalizerName = "runs.marmotdata.io/finalizer"

	annotationTrigger = "runs.marmotdata.io/trigger"

	conditionTypeReady = "Ready"

	requeueRunning = 30 * time.Second
	requeueIdle    = 5 * time.Minute
)

// RunReconciler reconciles a Run object.
type RunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config OperatorConfig
}

// +kubebuilder:rbac:groups=runs.marmotdata.io,resources=runs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=runs.marmotdata.io,resources=runs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=runs.marmotdata.io,resources=runs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles a single reconciliation loop for a Run resource.
func (r *RunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var run runsv1alpha1.Run
	if err := r.Get(ctx, req.NamespacedName, &run); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !run.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &run)
	}

	teardown := run.Spec.TeardownOnDelete == nil || *run.Spec.TeardownOnDelete
	if teardown {
		if !controllerutil.ContainsFinalizer(&run, finalizerName) {
			controllerutil.AddFinalizer(&run, finalizerName)
			if err := r.Update(ctx, &run); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(&run, finalizerName) {
			controllerutil.RemoveFinalizer(&run, finalizerName)
			if err := r.Update(ctx, &run); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	if err := r.reconcileConfigMap(ctx, &run); err != nil {
		logger.Error(err, "failed to reconcile ConfigMap")
		return ctrl.Result{}, err
	}

	var result ctrl.Result
	var err error
	if run.Spec.Schedule != "" {
		result, err = r.reconcileCronJob(ctx, &run)
	} else {
		result, err = r.reconcileJob(ctx, &run)
	}
	if err != nil {
		logger.Error(err, "failed to reconcile workload")
		return ctrl.Result{}, err
	}

	if err := r.updateStatus(ctx, &run); err != nil {
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	if run.Annotations[annotationTrigger] == "true" {
		if err := r.handleAnnotationTrigger(ctx, &run); err != nil {
			logger.Error(err, "failed to handle annotation trigger")
			return ctrl.Result{}, err
		}
	}

	return result, nil
}

func (r *RunReconciler) handleDeletion(ctx context.Context, run *runsv1alpha1.Run) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(run, finalizerName) {
		return ctrl.Result{}, nil
	}

	teardown := run.Spec.TeardownOnDelete == nil || *run.Spec.TeardownOnDelete
	if teardown {
		teardownJob := buildTeardownJob(run, r.Config)
		if err := ctrl.SetControllerReference(run, teardownJob, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting owner reference on teardown job: %w", err)
		}

		existing := &batchv1.Job{}
		err := r.Get(ctx, client.ObjectKeyFromObject(teardownJob), existing)
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, teardownJob); err != nil {
				return ctrl.Result{}, fmt.Errorf("creating teardown job: %w", err)
			}
			// Requeue to wait for job completion
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		} else if err != nil {
			return ctrl.Result{}, err
		}

		if existing.Status.Succeeded == 0 && existing.Status.Failed == 0 {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	controllerutil.RemoveFinalizer(run, finalizerName)
	if err := r.Update(ctx, run); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RunReconciler) reconcileConfigMap(ctx context.Context, run *runsv1alpha1.Run) error {
	desired := buildConfigMap(run)
	if err := ctrl.SetControllerReference(run, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on configmap: %w", err)
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update if data changed
	if !equality.Semantic.DeepEqual(existing.Data, desired.Data) {
		existing.Data = desired.Data
		return r.Update(ctx, existing)
	}
	return nil
}

func (r *RunReconciler) reconcileCronJob(ctx context.Context, run *runsv1alpha1.Run) (ctrl.Result, error) {
	desired := buildCronJob(run, r.Config)
	if err := ctrl.SetControllerReference(run, desired, r.Scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("setting owner reference on cronjob: %w", err)
	}

	existing := &batchv1.CronJob{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueIdle}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update mutable fields
	existing.Spec.Schedule = desired.Spec.Schedule
	existing.Spec.Suspend = desired.Spec.Suspend
	existing.Spec.ConcurrencyPolicy = desired.Spec.ConcurrencyPolicy
	existing.Spec.SuccessfulJobsHistoryLimit = desired.Spec.SuccessfulJobsHistoryLimit
	existing.Spec.FailedJobsHistoryLimit = desired.Spec.FailedJobsHistoryLimit
	existing.Spec.JobTemplate = desired.Spec.JobTemplate
	if err := r.Update(ctx, existing); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueIdle}, nil
}

func (r *RunReconciler) reconcileJob(ctx context.Context, run *runsv1alpha1.Run) (ctrl.Result, error) {
	desired := buildJob(run, r.Config)
	if err := ctrl.SetControllerReference(run, desired, r.Scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("setting owner reference on job: %w", err)
	}

	existing := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueRunning}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// For one-shot jobs, if spec generation changed, delete old and create new
	if run.Generation != run.Status.ObservedGeneration && run.Status.ObservedGeneration != 0 {
		if err := r.Delete(ctx, existing, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
	}

	if existing.Status.Active > 0 {
		return ctrl.Result{RequeueAfter: requeueRunning}, nil
	}
	return ctrl.Result{}, nil
}

func (r *RunReconciler) updateStatus(ctx context.Context, run *runsv1alpha1.Run) error {
	run.Status.ObservedGeneration = run.Generation

	if run.Spec.Schedule != "" {
		return r.updateStatusFromCronJob(ctx, run)
	}
	return r.updateStatusFromJob(ctx, run)
}

func (r *RunReconciler) updateStatusFromCronJob(ctx context.Context, run *runsv1alpha1.Run) error {
	var cj batchv1.CronJob
	if err := r.Get(ctx, client.ObjectKey{Namespace: run.Namespace, Name: run.Name}, &cj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	active := len(cj.Status.Active)
	if active > math.MaxInt32 {
		active = math.MaxInt32
	}
	run.Status.Active = int32(active) //nolint:gosec // bounds-checked above
	run.Status.LastRunTime = cj.Status.LastScheduleTime
	run.Status.LastSuccessfulTime = cj.Status.LastSuccessfulTime

	switch {
	case run.Spec.Suspend != nil && *run.Spec.Suspend:
		run.Status.Phase = runsv1alpha1.RunPhaseSuspended
	case run.Status.Active > 0:
		run.Status.Phase = runsv1alpha1.RunPhaseRunning
	default:
		run.Status.Phase = runsv1alpha1.RunPhaseScheduled
	}

	meta.SetStatusCondition(&run.Status.Conditions, metav1.Condition{
		Type:               conditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "CronJobCreated",
		Message:            fmt.Sprintf("CronJob %s is active", run.Name),
		ObservedGeneration: run.Generation,
	})

	return r.Status().Update(ctx, run)
}

func (r *RunReconciler) updateStatusFromJob(ctx context.Context, run *runsv1alpha1.Run) error {
	var job batchv1.Job
	if err := r.Get(ctx, client.ObjectKey{Namespace: run.Namespace, Name: run.Name}, &job); err != nil {
		if apierrors.IsNotFound(err) {
			run.Status.Phase = runsv1alpha1.RunPhaseIdle
			return r.Status().Update(ctx, run)
		}
		return err
	}

	run.Status.Active = job.Status.Active

	switch {
	case job.Status.Succeeded > 0:
		run.Status.Phase = runsv1alpha1.RunPhaseSucceeded
		now := metav1.Now()
		run.Status.LastSuccessfulTime = &now
	case job.Status.Failed > 0:
		run.Status.Phase = runsv1alpha1.RunPhaseFailed
	case job.Status.Active > 0:
		run.Status.Phase = runsv1alpha1.RunPhaseRunning
	default:
		run.Status.Phase = runsv1alpha1.RunPhaseIdle
	}

	reason := "JobCreated"
	status := metav1.ConditionTrue
	message := fmt.Sprintf("Job %s phase: %s", run.Name, run.Status.Phase)
	if run.Status.Phase == runsv1alpha1.RunPhaseFailed {
		reason = "JobFailed"
		status = metav1.ConditionFalse
	}

	meta.SetStatusCondition(&run.Status.Conditions, metav1.Condition{
		Type:               conditionTypeReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: run.Generation,
	})

	return r.Status().Update(ctx, run)
}

func (r *RunReconciler) handleAnnotationTrigger(ctx context.Context, run *runsv1alpha1.Run) error {
	logger := log.FromContext(ctx)
	logger.Info("handling annotation trigger", "run", run.Name)

	triggerJob := buildTriggerJob(run, r.Config)
	if err := ctrl.SetControllerReference(run, triggerJob, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on trigger job: %w", err)
	}

	existing := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKeyFromObject(triggerJob), existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, triggerJob); err != nil {
			return fmt.Errorf("creating trigger job: %w", err)
		}
		// Update LastRunTime for manual triggers
		now := metav1.Now()
		run.Status.LastRunTime = &now
		if err := r.Status().Update(ctx, run); err != nil {
			logger.Error(err, "failed to update LastRunTime for trigger")
		}
	} else if err != nil {
		return err
	}

	delete(run.Annotations, annotationTrigger)
	return r.Update(ctx, run)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runsv1alpha1.Run{}).
		Owns(&batchv1.CronJob{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
