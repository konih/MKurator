package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

const forceOrphanAnnotationValue = "true"

func mqObjectFrom(obj client.Object) (MQObject, error) {
	mo, ok := obj.(MQObject)
	if !ok {
		return nil, fmt.Errorf("unsupported workload type %T", obj)
	}
	return mo, nil
}

func updateMQStatusFields(
	mo MQObject,
	opts syncStatusOpts,
	message string,
	lastSync *metav1.Time,
) {
	fields := mo.GetMQStatusFields()
	fields.Message = message
	if lastSync != nil {
		fields.LastSyncTime = lastSync
	}
	if opts.mqObjectExists != nil {
		fields.MQObjectExists = opts.mqObjectExists
	}
}

type syncedStatusPatch struct {
	conditionStatus metav1.ConditionStatus
	reason          string
	generation      int64
	message         string
	opts            syncStatusOpts
	lastSync        *metav1.Time
	setObservedGen  bool
	emitEvent       bool
}

func patchSyncedStatus(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	patch syncedStatusPatch,
) error {
	mo, err := mqObjectFrom(obj)
	if err != nil {
		return fmt.Errorf("patch synced status: %w", err)
	}
	if patch.emitEvent {
		emitSyncedTransitionEvent(recorder, obj, patch.conditionStatus, patch.reason, patch.message)
	}
	setCondition(
		mo.GetMQConditions(),
		messagingv1alpha1.ConditionSynced,
		patch.conditionStatus,
		patch.reason,
		patch.message,
		patch.generation,
	)
	if patch.setObservedGen {
		mo.SetStatusObservedGeneration(patch.generation)
	}
	updateMQStatusFields(mo, patch.opts, patch.message, patch.lastSync)
	return status.Update(ctx, obj)
}

func resolveConnection(
	ctx context.Context,
	c client.Client,
	namespace, name string,
) (*messagingv1alpha1.QueueManagerConnection, error) {
	conn := &messagingv1alpha1.QueueManagerConnection{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, conn); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, &mqadmin.ConnectionNotFoundError{Name: name, Cause: err}
		}
		return nil, fmt.Errorf("get connection %q: %w", name, err)
	}
	return conn, nil
}

func waitForConnectionReady(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	conn *messagingv1alpha1.QueueManagerConnection,
	generation int64,
) (ctrl.Result, bool, error) {
	if connectionReady(conn) {
		return ctrl.Result{}, false, nil
	}
	msg := connectionWaitMessage(conn)
	if err := patchSyncedProgressing(ctx, status, recorder, obj, generation, msg); err != nil {
		return ctrl.Result{}, true, err
	}
	return ctrl.Result{RequeueAfter: ConnectionWaitInterval()}, true, nil
}

func syncedConditions(obj client.Object) []metav1.Condition {
	mo, err := mqObjectFrom(obj)
	if err != nil {
		return nil
	}
	return *mo.GetMQConditions()
}

func emitSyncedTransitionEvent(
	recorder events.EventRecorder,
	obj client.Object,
	newStatus metav1.ConditionStatus,
	newReason, message string,
) {
	if conditionChanged(syncedConditions(obj), messagingv1alpha1.ConditionSynced, newStatus, newReason) {
		recordNormalEvent(recorder, obj, newReason, message)
	}
}

func patchSyncedProgressing(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	message string,
) error {
	return patchSyncedStatus(ctx, status, recorder, obj, syncedStatusPatch{
		conditionStatus: metav1.ConditionFalse,
		reason:          messagingv1alpha1.ReasonProgressing,
		generation:      generation,
		message:         message,
		emitEvent:       true,
	})
}

func setSyncedError(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	err error,
	opts syncStatusOpts,
) (ctrl.Result, error) {
	recordReconcileWarning(recorder, obj, err)

	reason, message := classifyReconcileError(err)
	requeue := ctrl.Result{}
	if errors.Is(err, mqadmin.ErrTransient) {
		requeue = ctrl.Result{RequeueAfter: TransientRequeueInterval()}
	}

	if patchErr := patchSyncedStatus(ctx, status, recorder, obj, syncedStatusPatch{
		conditionStatus: metav1.ConditionFalse,
		reason:          reason,
		generation:      generation,
		message:         message,
		opts:            opts,
	}); patchErr != nil {
		return requeue, patchErr
	}

	if errors.Is(err, mqadmin.ErrTransient) {
		return requeue, nil
	}
	return ctrl.Result{}, nil
}

func patchSyncedAvailable(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	message string,
	opts syncStatusOpts,
) error {
	now := metav1.Now()
	return patchSyncedStatus(ctx, status, recorder, obj, syncedStatusPatch{
		conditionStatus: metav1.ConditionTrue,
		reason:          messagingv1alpha1.ReasonAvailable,
		generation:      generation,
		message:         message,
		opts:            opts,
		lastSync:        &now,
		setObservedGen:  true,
		emitEvent:       true,
	})
}

func patchSyncedDeleting(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	message string,
) error {
	return patchSyncedStatus(ctx, status, recorder, obj, syncedStatusPatch{
		conditionStatus: metav1.ConditionFalse,
		reason:          messagingv1alpha1.ReasonDeleting,
		generation:      generation,
		message:         message,
		emitEvent:       true,
	})
}

func forceOrphanRequested(obj metav1.Object) bool {
	ann := obj.GetAnnotations()
	return ann != nil && ann[messagingv1alpha1.ForceOrphanAnnotation] == forceOrphanAnnotationValue
}

func reconcileWorkloadDeletion(
	ctx context.Context,
	c client.Client,
	status client.StatusWriter,
	recorder events.EventRecorder,
	factory mqadmin.Factory,
	obj client.Object,
	generation int64,
	finalizer string,
	orphanMessage string,
	deleteFn func(ctx context.Context, admin mqadmin.Admin) (ctrl.Result, error),
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		return ctrl.Result{}, nil
	}

	if orphanDeletionRequested(obj) {
		return orphanFinalizeWorkload(ctx, c, status, recorder, obj, generation, finalizer, orphanMessage)
	}

	connRef, err := connectionRefName(obj)
	if err != nil {
		return ctrl.Result{}, err
	}
	conn, err := resolveConnection(ctx, c, obj.GetNamespace(), connRef)
	if err != nil {
		return deletionAwaitingConnection(ctx, status, recorder, obj, generation, err)
	}

	waitResult, waitDone, waitErr := waitForConnectionReady(ctx, status, recorder, obj, conn, generation)
	if waitDone {
		return waitResult, waitErr
	}

	admin, err := factory.ForConnection(ctx, conn)
	if err != nil {
		return deletionAwaitingConnection(ctx, status, recorder, obj, generation, err)
	}

	return deleteFn(ctx, admin)
}

func deletionAwaitingConnection(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	err error,
) (ctrl.Result, error) {
	msg := fmt.Sprintf("Deletion waiting for connection: %v", err)
	if patchErr := patchSyncedProgressing(ctx, status, recorder, obj, generation, msg); patchErr != nil {
		return ctrl.Result{RequeueAfter: ConnectionWaitInterval()}, patchErr
	}
	return ctrl.Result{RequeueAfter: ConnectionWaitInterval()}, nil
}

func orphanFinalizeWorkload(
	ctx context.Context,
	c client.Client,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	finalizer string,
	message string,
) (ctrl.Result, error) {
	if err := patchSyncedOrphaned(ctx, status, recorder, obj, generation, message); err != nil {
		return ctrl.Result{}, err
	}
	recordNormalEvent(recorder, obj, messagingv1alpha1.ReasonOrphaned, message)

	controllerutil.RemoveFinalizer(obj, finalizer)
	return ctrl.Result{}, c.Update(ctx, obj)
}

func patchSyncedOrphaned(
	ctx context.Context,
	status client.StatusWriter,
	recorder events.EventRecorder,
	obj client.Object,
	generation int64,
	message string,
) error {
	return patchSyncedStatus(ctx, status, recorder, obj, syncedStatusPatch{
		conditionStatus: metav1.ConditionFalse,
		reason:          messagingv1alpha1.ReasonOrphaned,
		generation:      generation,
		message:         message,
		emitEvent:       true,
	})
}

func connectionRefName(obj client.Object) (string, error) {
	mo, err := mqObjectFrom(obj)
	if err != nil {
		return "", err
	}
	return mo.ConnectionRefName(), nil
}

type mqWorkloadObject interface {
	MQObject
	client.Object
}

func appendListDependents[T mqWorkloadObject, L client.ObjectList](
	ctx context.Context,
	c client.Client,
	ns, connName, kind string,
	newList func() L,
	itemsFn func(L) []T,
	reqs []reconcile.Request,
) ([]reconcile.Request, error) {
	list := newList()
	if err := c.List(ctx, list, client.InNamespace(ns)); err != nil {
		return reqs, fmt.Errorf("list %s: %w", kind, err)
	}
	for _, item := range itemsFn(list) {
		if item.ConnectionRefName() == connName {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: ns, Name: item.GetName()},
			})
		}
	}
	return reqs, nil
}

func appendDependentsOrLog[T mqWorkloadObject, L client.ObjectList](
	ctx context.Context,
	c client.Client,
	logger logr.Logger,
	ns, connName, kind string,
	newList func() L,
	itemsFn func(L) []T,
	reqs []reconcile.Request,
) []reconcile.Request {
	added, err := appendListDependents(ctx, c, ns, connName, kind, newList, itemsFn, reqs)
	if err != nil {
		logger.Error(err, "list dependent resources for connection fan-out",
			"namespace", ns, "connection", connName, "kind", kind)
		return reqs
	}
	return added
}

func mqObjectItems[T mqWorkloadObject, S ~[]E, E any](items S, ptr func(*E) T) []T {
	out := make([]T, len(items))
	for i := range items {
		out[i] = ptr(&items[i])
	}
	return out
}

func requestsForConnection(
	ctx context.Context,
	c client.Client,
	conn *messagingv1alpha1.QueueManagerConnection,
) []reconcile.Request {
	logger := log.FromContext(ctx)
	ns := conn.Namespace
	connName := conn.Name
	var reqs []reconcile.Request

	reqs = appendDependentsOrLog(ctx, c, logger, ns, connName, "Queue",
		func() *messagingv1alpha1.QueueList { return &messagingv1alpha1.QueueList{} },
		func(l *messagingv1alpha1.QueueList) []*messagingv1alpha1.Queue {
			return mqObjectItems(l.Items, func(q *messagingv1alpha1.Queue) *messagingv1alpha1.Queue { return q })
		},
		reqs,
	)
	reqs = appendDependentsOrLog(ctx, c, logger, ns, connName, "Topic",
		func() *messagingv1alpha1.TopicList { return &messagingv1alpha1.TopicList{} },
		func(l *messagingv1alpha1.TopicList) []*messagingv1alpha1.Topic {
			return mqObjectItems(l.Items, func(t *messagingv1alpha1.Topic) *messagingv1alpha1.Topic { return t })
		},
		reqs,
	)
	reqs = appendDependentsOrLog(ctx, c, logger, ns, connName, "Channel",
		func() *messagingv1alpha1.ChannelList { return &messagingv1alpha1.ChannelList{} },
		func(l *messagingv1alpha1.ChannelList) []*messagingv1alpha1.Channel {
			return mqObjectItems(l.Items, func(ch *messagingv1alpha1.Channel) *messagingv1alpha1.Channel { return ch })
		},
		reqs,
	)
	reqs = appendDependentsOrLog(ctx, c, logger, ns, connName, "ChannelAuthRule",
		func() *messagingv1alpha1.ChannelAuthRuleList { return &messagingv1alpha1.ChannelAuthRuleList{} },
		func(l *messagingv1alpha1.ChannelAuthRuleList) []*messagingv1alpha1.ChannelAuthRule {
			return mqObjectItems(
				l.Items,
				func(r *messagingv1alpha1.ChannelAuthRule) *messagingv1alpha1.ChannelAuthRule {
					return r
				},
			)
		},
		reqs,
	)
	reqs = appendDependentsOrLog(ctx, c, logger, ns, connName, "AuthorityRecord",
		func() *messagingv1alpha1.AuthorityRecordList { return &messagingv1alpha1.AuthorityRecordList{} },
		func(l *messagingv1alpha1.AuthorityRecordList) []*messagingv1alpha1.AuthorityRecord {
			return mqObjectItems(
				l.Items,
				func(a *messagingv1alpha1.AuthorityRecord) *messagingv1alpha1.AuthorityRecord {
					return a
				},
			)
		},
		reqs,
	)

	return reqs
}

func connectionEnqueueMapper(c client.Client) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		conn, ok := obj.(*messagingv1alpha1.QueueManagerConnection)
		if !ok {
			return nil
		}
		return requestsForConnection(ctx, c, conn)
	}
}

func watchConnectionStatus(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(connectionEnqueueMapper(c))
}

func connectionReady(conn *messagingv1alpha1.QueueManagerConnection) bool {
	for _, c := range conn.Status.Conditions {
		if c.Type == messagingv1alpha1.ConditionReady && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func connectionReadyChanged(oldConn, newConn *messagingv1alpha1.QueueManagerConnection) bool {
	return connectionReady(oldConn) != connectionReady(newConn)
}

func connectionWatchPredicates() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			conn, ok := e.Object.(*messagingv1alpha1.QueueManagerConnection)
			return ok && connectionReady(conn)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldConn, okOld := e.ObjectOld.(*messagingv1alpha1.QueueManagerConnection)
			newConn, okNew := e.ObjectNew.(*messagingv1alpha1.QueueManagerConnection)
			if !okOld || !okNew {
				return false
			}
			return connectionReadyChanged(oldConn, newConn) || oldConn.Generation != newConn.Generation
		},
	}
}

func setupMQObjectController(mgr ctrl.Manager, reconciler reconcile.Reconciler, forObj client.Object) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(forObj, builder.WithPredicates(workloadReconcilePredicates())).
		WithOptions(controllerOptions()).
		Watches(
			&messagingv1alpha1.QueueManagerConnection{},
			watchConnectionStatus(mgr.GetClient()),
			builder.WithPredicates(connectionWatchPredicates()),
		).
		Complete(reconciler)
}
