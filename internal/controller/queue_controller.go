package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	messagingv1alpha1 "github.com/konradheimel/kurator/api/v1alpha1"
	"github.com/konradheimel/kurator/internal/metrics"
	"github.com/konradheimel/kurator/internal/mqadmin"
)

// QueueReconciler reconciles Queue objects into MQSC on IBM MQ.
type QueueReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
}

// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queues/finalizers,verbs=update
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queuemanagerconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile ensures the MQ queue matches spec.
func (r *QueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	result, err := r.reconcile(ctx, req)
	metrics.RecordReconcile(metrics.ControllerQueue, err)
	return result, err
}

//nolint:dupl // same connection/finalizer/sync pattern as TopicReconciler
func (r *QueueReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	q := &messagingv1alpha1.Queue{}
	if err := r.Get(ctx, req.NamespacedName, q); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get Queue: %w", err)
	}

	conn, err := r.getConnection(ctx, q)
	if err != nil {
		return r.setSyncedError(ctx, q, err)
	}

	if !connectionReady(conn) {
		setCondition(&q.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, messagingv1alpha1.ReasonProgressing,
			fmt.Sprintf("waiting for connection %q to become Ready", conn.Name), q.Generation)
		if statusErr := r.Status().Update(ctx, q); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	admin, err := r.MQFactory.ForConnection(ctx, conn)
	if err != nil {
		return r.setSyncedError(ctx, q, err)
	}

	if !q.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, q, admin)
	}

	if !controllerutil.ContainsFinalizer(q, messagingv1alpha1.QueueFinalizer) {
		controllerutil.AddFinalizer(q, messagingv1alpha1.QueueFinalizer)
		if err := r.Update(ctx, q); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	spec := toMQQueueSpec(q)
	if err := r.ensureQueue(ctx, admin, spec); err != nil {
		return r.setSyncedError(ctx, q, err)
	}

	setCondition(&q.Status.Conditions, messagingv1alpha1.ConditionSynced,
		metav1.ConditionTrue, messagingv1alpha1.ReasonAvailable, "Queue matches spec", q.Generation)
	q.Status.ObservedGeneration = q.Generation
	if err := r.Status().Update(ctx, q); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("Queue synced", "queue", q.Spec.QueueName)
	return ctrl.Result{}, nil
}

func (r *QueueReconciler) getConnection(
	ctx context.Context,
	q *messagingv1alpha1.Queue,
) (*messagingv1alpha1.QueueManagerConnection, error) {
	conn := &messagingv1alpha1.QueueManagerConnection{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: q.Namespace,
		Name:      q.Spec.ConnectionRef.Name,
	}, conn); err != nil {
		return nil, fmt.Errorf("get connection %q: %w", q.Spec.ConnectionRef.Name, err)
	}
	return conn, nil
}

func (r *QueueReconciler) ensureQueue(ctx context.Context, admin mqadmin.Admin, spec mqadmin.QueueSpec) error {
	observed, err := admin.GetQueue(ctx, spec)
	if err != nil && !errors.Is(err, mqadmin.ErrNotFound) {
		return err
	}
	if observed == nil || needsUpdate(spec, observed) {
		if err := admin.DefineQueue(ctx, spec); err != nil {
			return err
		}
	}
	return nil
}

func needsUpdate(desired mqadmin.QueueSpec, observed *mqadmin.QueueState) bool {
	return mqadmin.AttributesNeedUpdate(desired.Attributes, observed.Attributes)
}

func (r *QueueReconciler) handleDeletion(
	ctx context.Context,
	q *messagingv1alpha1.Queue,
	admin mqadmin.Admin,
) (ctrl.Result, error) {
	setCondition(&q.Status.Conditions, messagingv1alpha1.ConditionSynced,
		metav1.ConditionFalse, messagingv1alpha1.ReasonDeleting, "Deleting queue from IBM MQ", q.Generation)
	if err := r.Status().Update(ctx, q); err != nil {
		return ctrl.Result{}, err
	}

	if err := admin.DeleteQueue(ctx, toMQQueueSpec(q)); err != nil {
		return r.setSyncedError(ctx, q, err)
	}

	controllerutil.RemoveFinalizer(q, messagingv1alpha1.QueueFinalizer)
	if err := r.Update(ctx, q); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *QueueReconciler) setSyncedError(
	ctx context.Context,
	q *messagingv1alpha1.Queue,
	err error,
) (ctrl.Result, error) {
	reason := messagingv1alpha1.ReasonError
	requeue := ctrl.Result{}
	if errors.Is(err, mqadmin.ErrTransient) {
		requeue = ctrl.Result{RequeueAfter: 30 * time.Second}
	}
	setCondition(&q.Status.Conditions, messagingv1alpha1.ConditionSynced,
		metav1.ConditionFalse, reason, err.Error(), q.Generation)
	if statusErr := r.Status().Update(ctx, q); statusErr != nil {
		return requeue, statusErr
	}
	if errors.Is(err, mqadmin.ErrTransient) {
		return requeue, err
	}
	return ctrl.Result{}, nil
}

func connectionReady(conn *messagingv1alpha1.QueueManagerConnection) bool {
	for _, c := range conn.Status.Conditions {
		if c.Type == messagingv1alpha1.ConditionReady && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func toMQQueueSpec(q *messagingv1alpha1.Queue) mqadmin.QueueSpec {
	attrs := map[string]string{}
	for k, v := range q.Spec.Attributes {
		attrs[strings.ToLower(k)] = v
	}
	return mqadmin.QueueSpec{
		Name:       q.Spec.QueueName,
		Type:       mqadmin.QueueType(q.Spec.Type),
		Attributes: attrs,
	}
}

// SetupWithManager wires the reconciler.
func (r *QueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&messagingv1alpha1.Queue{}).
		Complete(r)
}
