package controller

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	messagingv1alpha1 "github.com/konih/mkurator/api/v1alpha1"
	"github.com/konih/mkurator/internal/metrics"
	"github.com/konih/mkurator/internal/mqadmin"
)

// QueueManagerConnectionReconciler reconciles QueueManagerConnection objects.
type QueueManagerConnectionReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
	Recorder  events.EventRecorder
}

// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queuemanagerconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queuemanagerconnections,verbs=create;update
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queuemanagerconnections,verbs=patch;delete
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queuemanagerconnections/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queuemanagerconnections/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile tests connectivity to mqweb and sets Ready.
func (r *QueueManagerConnectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	result, err := r.reconcile(ctx, req)
	metrics.RecordReconcile(metrics.ControllerQueueManagerConnection, err)
	return result, err
}

func (r *QueueManagerConnectionReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	conn := &messagingv1alpha1.QueueManagerConnection{}
	if err := r.Get(ctx, req.NamespacedName, conn); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get QueueManagerConnection: %w", err)
	}

	if !conn.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(conn, messagingv1alpha1.QueueManagerConnectionFinalizer) {
			if err := r.MQFactory.ReleaseConnection(ctx, conn); err != nil {
				return ctrl.Result{}, fmt.Errorf("release mq client: %w", err)
			}
			controllerutil.RemoveFinalizer(conn, messagingv1alpha1.QueueManagerConnectionFinalizer)
			if err := r.Update(ctx, conn); err != nil {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(conn, messagingv1alpha1.QueueManagerConnectionFinalizer) {
		controllerutil.AddFinalizer(conn, messagingv1alpha1.QueueManagerConnectionFinalizer)
		if err := r.Update(ctx, conn); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	gen := conn.Generation
	readyMsg := "mqweb connection is healthy"
	alreadyReady := connectionReady(conn) && conn.Status.ObservedGeneration == gen

	if !alreadyReady {
		setCondition(&conn.Status.Conditions, messagingv1alpha1.ConditionReady,
			metav1.ConditionFalse, messagingv1alpha1.ReasonProgressing, "Testing mqweb connectivity", gen)
		if err := r.Status().Update(ctx, conn); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status progressing: %w", err)
		}
	}

	admin, err := r.MQFactory.ForConnection(ctx, conn)
	if err != nil {
		return r.fail(ctx, conn, gen, err)
	}
	if err := admin.Ping(ctx); err != nil {
		return r.fail(ctx, conn, gen, err)
	}

	if alreadyReady {
		logger.V(1).Info("QueueManagerConnection still ready", "connection", conn.Name)
		return ctrl.Result{}, nil
	}

	if conditionChanged(conn.Status.Conditions, messagingv1alpha1.ConditionReady,
		metav1.ConditionTrue, messagingv1alpha1.ReasonAvailable) {
		recordNormalEvent(r.Recorder, conn, messagingv1alpha1.ReasonAvailable, readyMsg)
	}
	setCondition(&conn.Status.Conditions, messagingv1alpha1.ConditionReady,
		metav1.ConditionTrue, messagingv1alpha1.ReasonAvailable, readyMsg, gen)
	conn.Status.ObservedGeneration = gen
	if err := r.Status().Update(ctx, conn); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("QueueManagerConnection ready", "connection", conn.Name)
	return ctrl.Result{}, nil
}

func (r *QueueManagerConnectionReconciler) fail(
	ctx context.Context,
	conn *messagingv1alpha1.QueueManagerConnection,
	gen int64,
	err error,
) (ctrl.Result, error) {
	if connectionReady(conn) && conn.Status.ObservedGeneration == gen && errors.Is(err, mqadmin.ErrTransient) {
		return ctrl.Result{RequeueAfter: TransientRequeueInterval()}, nil
	}

	recordReconcileWarning(r.Recorder, conn, err)

	reason := messagingv1alpha1.ReasonError
	msg := err.Error()
	var requeue ctrl.Result
	if errors.Is(err, mqadmin.ErrTransient) {
		requeue = ctrl.Result{RequeueAfter: TransientRequeueInterval()}
	}
	setCondition(&conn.Status.Conditions, messagingv1alpha1.ConditionReady,
		metav1.ConditionFalse, reason, msg, gen)
	if statusErr := r.Status().Update(ctx, conn); statusErr != nil {
		return requeue, fmt.Errorf("update status: %w", statusErr)
	}
	if errors.Is(err, mqadmin.ErrTransient) {
		return requeue, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager wires the reconciler.
func (r *QueueManagerConnectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&messagingv1alpha1.QueueManagerConnection{}).
		WithOptions(controllerOptions()).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(secretEnqueueMapper(mgr.GetClient())),
			builder.WithPredicates(secretWatchPredicates()),
		).
		Complete(r)
}
