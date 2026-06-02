package controller

import (
	"context"
	"errors"
	"fmt"
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

// TopicReconciler reconciles Topic objects into MQSC on IBM MQ.
type TopicReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
}

// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=topics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=topics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=topics/finalizers,verbs=update
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queuemanagerconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile ensures the MQ topic matches spec.
func (r *TopicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	result, err := r.reconcile(ctx, req)
	metrics.RecordReconcile(metrics.ControllerTopic, err)
	return result, err
}

//nolint:dupl // same connection/finalizer/sync pattern as QueueReconciler
func (r *TopicReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	topic := &messagingv1alpha1.Topic{}
	if err := r.Get(ctx, req.NamespacedName, topic); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get Topic: %w", err)
	}

	conn, err := r.getConnection(ctx, topic)
	if err != nil {
		return r.setSyncedError(ctx, topic, err)
	}

	if !connectionReady(conn) {
		setCondition(&topic.Status.Conditions, messagingv1alpha1.ConditionSynced,
			metav1.ConditionFalse, messagingv1alpha1.ReasonProgressing,
			fmt.Sprintf("waiting for connection %q to become Ready", conn.Name), topic.Generation)
		if statusErr := r.Status().Update(ctx, topic); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	admin, err := r.MQFactory.ForConnection(ctx, conn)
	if err != nil {
		return r.setSyncedError(ctx, topic, err)
	}

	if !topic.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, topic, admin)
	}

	if !controllerutil.ContainsFinalizer(topic, messagingv1alpha1.TopicFinalizer) {
		controllerutil.AddFinalizer(topic, messagingv1alpha1.TopicFinalizer)
		if err := r.Update(ctx, topic); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	spec := toMQTopicSpec(topic)
	if err := r.ensureTopic(ctx, admin, spec); err != nil {
		return r.setSyncedError(ctx, topic, err)
	}

	setCondition(&topic.Status.Conditions, messagingv1alpha1.ConditionSynced,
		metav1.ConditionTrue, messagingv1alpha1.ReasonAvailable, "Topic matches spec", topic.Generation)
	topic.Status.ObservedGeneration = topic.Generation
	if err := r.Status().Update(ctx, topic); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("Topic synced", "topic", topic.Spec.TopicName)
	return ctrl.Result{}, nil
}

func (r *TopicReconciler) getConnection(
	ctx context.Context,
	topic *messagingv1alpha1.Topic,
) (*messagingv1alpha1.QueueManagerConnection, error) {
	conn := &messagingv1alpha1.QueueManagerConnection{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: topic.Namespace,
		Name:      topic.Spec.ConnectionRef.Name,
	}, conn); err != nil {
		return nil, fmt.Errorf("get connection %q: %w", topic.Spec.ConnectionRef.Name, err)
	}
	return conn, nil
}

func (r *TopicReconciler) ensureTopic(ctx context.Context, admin mqadmin.Admin, spec mqadmin.TopicSpec) error {
	observed, err := admin.GetTopic(ctx, spec.Name)
	if err != nil && !errors.Is(err, mqadmin.ErrNotFound) {
		return err
	}
	if observed == nil || topicNeedsUpdate(spec, observed) {
		if err := admin.DefineTopic(ctx, spec); err != nil {
			return err
		}
	}
	return nil
}

func topicNeedsUpdate(desired mqadmin.TopicSpec, observed *mqadmin.TopicState) bool {
	return mqadmin.AttributesNeedUpdate(desired.Attributes, observed.Attributes)
}

func (r *TopicReconciler) handleDeletion(
	ctx context.Context,
	topic *messagingv1alpha1.Topic,
	admin mqadmin.Admin,
) (ctrl.Result, error) {
	setCondition(&topic.Status.Conditions, messagingv1alpha1.ConditionSynced,
		metav1.ConditionFalse, messagingv1alpha1.ReasonDeleting, "Deleting topic from IBM MQ", topic.Generation)
	if err := r.Status().Update(ctx, topic); err != nil {
		return ctrl.Result{}, err
	}

	if err := admin.DeleteTopic(ctx, topic.Spec.TopicName); err != nil {
		return r.setSyncedError(ctx, topic, err)
	}

	controllerutil.RemoveFinalizer(topic, messagingv1alpha1.TopicFinalizer)
	if err := r.Update(ctx, topic); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *TopicReconciler) setSyncedError(
	ctx context.Context,
	topic *messagingv1alpha1.Topic,
	err error,
) (ctrl.Result, error) {
	reason := messagingv1alpha1.ReasonError
	requeue := ctrl.Result{}
	if errors.Is(err, mqadmin.ErrTransient) {
		requeue = ctrl.Result{RequeueAfter: 30 * time.Second}
	}
	setCondition(&topic.Status.Conditions, messagingv1alpha1.ConditionSynced,
		metav1.ConditionFalse, reason, err.Error(), topic.Generation)
	if statusErr := r.Status().Update(ctx, topic); statusErr != nil {
		return requeue, statusErr
	}
	if errors.Is(err, mqadmin.ErrTransient) {
		return requeue, err
	}
	return ctrl.Result{}, nil
}

func toMQTopicSpec(topic *messagingv1alpha1.Topic) mqadmin.TopicSpec {
	attrs := map[string]string{}
	for k, v := range topic.Spec.Attributes {
		attrs[mqadmin.NormalizeAttrKey(k)] = v
	}
	return mqadmin.TopicSpec{
		Name:       topic.Spec.TopicName,
		Attributes: attrs,
	}
}

// SetupWithManager wires the reconciler.
func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&messagingv1alpha1.Topic{}).
		Complete(r)
}
