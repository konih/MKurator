package controller

import (
	"context"
	"errors"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/metrics"
	"github.com/konih/kurator/internal/mqadmin"
)

// TopicReconciler reconciles Topic objects into MQSC on IBM MQ.
type TopicReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
	Recorder  record.EventRecorder
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

//nolint:dupl // shared MQ object reconcile flow; differs in ensure/delete/spec mapping
func (r *TopicReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	topic := &messagingv1alpha1.Topic{}
	if err := r.Get(ctx, req.NamespacedName, topic); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get Topic: %w", err)
	}

	connRef, err := connectionRefName(topic)
	if err != nil {
		return ctrl.Result{}, err
	}
	conn, err := resolveConnection(ctx, r.Client, topic.Namespace, connRef)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, topic, topic.Generation, err)
	}

	waitResult, waitDone, waitErr := waitForConnectionReady(ctx, r.Status(), r.Recorder, topic, conn, topic.Generation)
	if waitDone {
		return waitResult, waitErr
	}

	admin, err := r.MQFactory.ForConnection(ctx, conn)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, topic, topic.Generation, err)
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
		return setSyncedError(ctx, r.Status(), r.Recorder, topic, topic.Generation, err)
	}

	if err := patchSyncedAvailable(ctx, r.Status(), r.Recorder, topic, topic.Generation,
		"Topic matches spec"); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("Topic synced", "topic", topic.Spec.TopicName)
	return ctrl.Result{}, nil
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
	if err := patchSyncedDeleting(ctx, r.Status(), r.Recorder, topic, topic.Generation,
		"Deleting topic from IBM MQ"); err != nil {
		return ctrl.Result{}, err
	}

	if err := admin.DeleteTopic(ctx, topic.Spec.TopicName); err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, topic, topic.Generation, err)
	}

	recordNormalEvent(r.Recorder, topic, EventReasonDeleted, "Topic removed from IBM MQ")

	controllerutil.RemoveFinalizer(topic, messagingv1alpha1.TopicFinalizer)
	if err := r.Update(ctx, topic); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
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
	return setupMQObjectController(mgr, r, &messagingv1alpha1.Topic{})
}
