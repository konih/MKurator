package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
	"github.com/conduit-ops/mkurator/internal/metrics"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

// QueueReconciler reconciles Queue objects into MQSC on IBM MQ.
type QueueReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
	Recorder  events.EventRecorder
}

// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queues/finalizers,verbs=update
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queuemanagerconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile ensures the MQ queue matches spec.
func (r *QueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	result, err := r.reconcile(ctx, req)
	metrics.RecordReconcile(metrics.ControllerQueue, err)
	return result, err
}

//nolint:dupl // shared MQ object reconcile flow; differs in ensure/delete/spec mapping
func (r *QueueReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	q := &messagingv1alpha1.Queue{}
	if err := r.Get(ctx, req.NamespacedName, q); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get Queue: %w", err)
	}

	if workloadSuspended(q) {
		return reconcileWorkloadSuspended(ctx, r.Status(), r.Recorder, q, q.Generation)
	}

	if !q.DeletionTimestamp.IsZero() {
		return reconcileWorkloadDeletion(
			ctx, r.Client, r.Status(), r.Recorder, r.MQFactory, q, q.Generation,
			messagingv1alpha1.QueueFinalizer, "Queue orphaned in IBM MQ",
			func(ctx context.Context, admin mqadmin.Admin) (ctrl.Result, error) {
				return r.handleDeletion(ctx, q, admin)
			},
		)
	}

	connRef, err := connectionRefName(q)
	if err != nil {
		return ctrl.Result{}, err
	}
	conn, err := resolveConnection(ctx, r.Client, q.Namespace, connRef)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, q, q.Generation, err, syncStatusOpts{})
	}

	waitResult, waitDone, waitErr := waitForConnectionReady(ctx, r.Status(), r.Recorder, q, conn, q.Generation)
	if waitDone {
		return waitResult, waitErr
	}

	admin, err := r.MQFactory.ForConnection(ctx, conn)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, q, q.Generation, err, syncStatusOpts{})
	}

	if !controllerutil.ContainsFinalizer(q, messagingv1alpha1.QueueFinalizer) {
		controllerutil.AddFinalizer(q, messagingv1alpha1.QueueFinalizer)
		if updateErr := r.Update(ctx, q); updateErr != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", updateErr)
		}
		return ctrl.Result{}, nil
	}

	spec := toMQQueueSpec(q)
	desiredMQSC, formatErr := mqrest.FormatDefineQueueMQSC(spec)
	if formatErr != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, q, q.Generation, formatErr, syncStatusOpts{})
	}
	q.Status.DesiredMQSC = desiredMQSC

	mqExists, driftMsg, err := r.ensureQueue(ctx, admin, q, spec, isObserveOnly(q))
	if err != nil {
		var block *AdoptionBlockedError
		if errors.As(err, &block) {
			return handleAdoptionBlock(ctx, r.Status(), r.Recorder, q, q.Generation, block,
				syncStatusOpts{mqObjectExists: &mqExists})
		}
		return setSyncedError(
			ctx,
			r.Status(),
			r.Recorder,
			q,
			q.Generation,
			err,
			syncStatusOpts{mqObjectExists: &mqExists},
		)
	}
	if driftMsg != "" {
		metrics.RecordDriftDetected(metrics.ControllerQueue)
		opts := syncStatusOpts{mqObjectExists: boolPtr(mqExists)}
		if patchErr := patchSyncedDrift(ctx, r.Status(), r.Recorder, q, q.Generation, driftMsg, opts); patchErr != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", patchErr)
		}
		return workloadDriftResyncResult(), nil
	}

	if err := patchSyncedAvailable(ctx, r.Status(), r.Recorder, q, q.Generation, "Queue matches spec",
		syncStatusOpts{mqObjectExists: boolPtr(true)}); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("Queue synced", "queue", q.Spec.QueueName)
	return workloadDriftResyncResult(), nil
}

func (r *QueueReconciler) ensureQueue(
	ctx context.Context,
	admin mqadmin.Admin,
	q *messagingv1alpha1.Queue,
	spec mqadmin.QueueSpec,
	observeOnly bool,
) (bool, string, error) {
	mqCtx, cancel := MQRequestContext(ctx)
	defer cancel()

	observed, err := admin.GetQueue(mqCtx, spec)
	if err != nil && !errors.Is(err, mqadmin.ErrNotFound) {
		return false, "", err
	}
	exists := observed != nil
	var observedAttrs map[string]string
	if observed != nil {
		observedAttrs = observed.Attributes
	}
	return reconcileMQObjectState(
		observeOnly,
		workloadAdoptionPolicy(q),
		workloadFirstAdoption(q),
		exists,
		observedAttrs,
		spec.Attributes,
		mqrest.QueueDriftCheckKeys(spec.Type),
		fmt.Sprintf("queue %q", spec.Name),
		func() error { return admin.DefineQueue(mqCtx, spec) },
	)
}

func (r *QueueReconciler) handleDeletion(
	ctx context.Context,
	q *messagingv1alpha1.Queue,
	admin mqadmin.Admin,
) (ctrl.Result, error) {
	if err := patchSyncedDeleting(ctx, r.Status(), r.Recorder, q, q.Generation, "Deleting queue from IBM MQ"); err != nil {
		return ctrl.Result{}, err
	}

	mqCtx, cancel := MQRequestContext(ctx)
	defer cancel()

	if err := admin.DeleteQueue(mqCtx, toMQQueueSpec(q)); err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, q, q.Generation, err, syncStatusOpts{})
	}

	recordNormalEvent(r.Recorder, q, EventReasonDeleted, "Queue removed from IBM MQ")

	controllerutil.RemoveFinalizer(q, messagingv1alpha1.QueueFinalizer)
	if err := r.Update(ctx, q); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

func toMQQueueSpec(q *messagingv1alpha1.Queue) mqadmin.QueueSpec {
	attrs := map[string]string{}
	for k, v := range q.Spec.Attributes {
		attrs[mqadmin.NormalizeAttrKey(k)] = v
	}
	if q.Spec.MaxDepth != nil {
		attrs[mqadmin.NormalizeAttrKey("maxdepth")] = strconv.FormatInt(int64(*q.Spec.MaxDepth), 10)
	}
	if q.Spec.Description != "" {
		attrs[mqadmin.NormalizeAttrKey("descr")] = q.Spec.Description
	}
	if q.Spec.DefPersistence != "" {
		attrs[mqadmin.NormalizeAttrKey("defpsist")] = string(q.Spec.DefPersistence)
	}
	if q.Spec.Get != "" {
		attrs[mqadmin.NormalizeAttrKey("get")] = string(q.Spec.Get)
	}
	if q.Spec.Put != "" {
		attrs[mqadmin.NormalizeAttrKey("put")] = string(q.Spec.Put)
	}
	if q.Spec.TargetQueue != "" {
		attrs[mqadmin.NormalizeAttrKey("targq")] = q.Spec.TargetQueue
	}
	if q.Spec.XmitQueue != "" {
		attrs[mqadmin.NormalizeAttrKey("xmitq")] = q.Spec.XmitQueue
	}
	if q.Spec.RemoteQueueManager != "" {
		attrs[mqadmin.NormalizeAttrKey("rqmname")] = q.Spec.RemoteQueueManager
	}
	return mqadmin.QueueSpec{
		Name:       q.Spec.QueueName,
		Type:       mqadmin.QueueType(q.Spec.Type),
		Attributes: attrs,
	}
}

// SetupWithManager wires the reconciler.
func (r *QueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return setupMQObjectController(mgr, r, &messagingv1alpha1.Queue{})
}
