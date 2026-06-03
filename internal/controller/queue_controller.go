package controller

import (
	"context"
	"errors"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/adapter/mqrest"
	"github.com/konih/kurator/internal/metrics"
	"github.com/konih/kurator/internal/mqadmin"
)

// QueueReconciler reconciles Queue objects into MQSC on IBM MQ.
type QueueReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
	Recorder  events.EventRecorder
}

// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queues/finalizers,verbs=update
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queuemanagerconnections,verbs=get;list;watch
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

	if !q.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, q, admin)
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

	mqExists, driftMsg, err := r.ensureQueue(ctx, admin, spec, isObserveOnly(q))
	if err != nil {
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
		opts := syncStatusOpts{mqObjectExists: boolPtr(mqExists)}
		if patchErr := patchSyncedDrift(ctx, r.Status(), r.Recorder, q, q.Generation, driftMsg, opts); patchErr != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", patchErr)
		}
		return ctrl.Result{}, nil
	}

	if err := patchSyncedAvailable(ctx, r.Status(), r.Recorder, q, q.Generation, "Queue matches spec",
		syncStatusOpts{mqObjectExists: boolPtr(true)}); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("Queue synced", "queue", q.Spec.QueueName)
	return ctrl.Result{}, nil
}

func (r *QueueReconciler) ensureQueue(
	ctx context.Context,
	admin mqadmin.Admin,
	spec mqadmin.QueueSpec,
	observeOnly bool,
) (bool, string, error) {
	observed, err := admin.GetQueue(ctx, spec)
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
		exists,
		observedAttrs,
		spec.Attributes,
		mqrest.QueueDriftCheckKeys(spec.Type),
		fmt.Sprintf("queue %q", spec.Name),
		func() error { return admin.DefineQueue(ctx, spec) },
	)
}

func needsUpdate(desired mqadmin.QueueSpec, observed *mqadmin.QueueState) bool {
	return mqadmin.AttributesNeedUpdate(
		desired.Attributes,
		observed.Attributes,
		mqrest.QueueDriftCheckKeys(desired.Type),
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

	if err := admin.DeleteQueue(ctx, toMQQueueSpec(q)); err != nil {
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
