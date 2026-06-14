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

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
	"github.com/conduit-ops/mkurator/internal/metrics"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

// TopicReconciler reconciles Topic objects into MQSC on IBM MQ.
type TopicReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
	Recorder  events.EventRecorder
}

// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=topics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=topics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=topics/finalizers,verbs=update
// +kubebuilder:rbac:groups=messaging.mkurator.dev,resources=queuemanagerconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

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

	if workloadSuspended(topic) {
		return reconcileWorkloadSuspended(ctx, r.Status(), r.Recorder, topic, topic.Generation)
	}

	if !topic.DeletionTimestamp.IsZero() {
		return reconcileWorkloadDeletion(
			ctx, r.Client, r.Status(), r.Recorder, r.MQFactory, topic, topic.Generation,
			messagingv1alpha1.TopicFinalizer, "Topic orphaned in IBM MQ",
			func(ctx context.Context, admin mqadmin.Admin) (ctrl.Result, error) {
				return r.handleDeletion(ctx, topic, admin)
			},
		)
	}

	connRef, err := connectionRefName(topic)
	if err != nil {
		return ctrl.Result{}, err
	}
	conn, err := resolveConnection(ctx, r.Client, topic.Namespace, connRef)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, topic, topic.Generation, err, syncStatusOpts{})
	}

	waitResult, waitDone, waitErr := waitForConnectionReady(ctx, r.Status(), r.Recorder, topic, conn, topic.Generation)
	if waitDone {
		return waitResult, waitErr
	}

	admin, err := r.MQFactory.ForConnection(ctx, conn)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, topic, topic.Generation, err, syncStatusOpts{})
	}

	if !controllerutil.ContainsFinalizer(topic, messagingv1alpha1.TopicFinalizer) {
		controllerutil.AddFinalizer(topic, messagingv1alpha1.TopicFinalizer)
		if updateErr := r.Update(ctx, topic); updateErr != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", updateErr)
		}
		return ctrl.Result{}, nil
	}

	spec := toMQTopicSpec(topic)
	desiredMQSC, formatErr := mqrest.FormatDefineTopicMQSC(spec)
	if formatErr != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, topic, topic.Generation, formatErr, syncStatusOpts{})
	}
	topic.Status.DesiredMQSC = desiredMQSC

	mqExists, driftMsg, err := r.ensureTopic(ctx, admin, topic, spec, isObserveOnly(topic))
	if err != nil {
		var block *AdoptionBlockedError
		if errors.As(err, &block) {
			return handleAdoptionBlock(ctx, r.Status(), r.Recorder, topic, topic.Generation, block,
				syncStatusOpts{mqObjectExists: &mqExists})
		}
		return setSyncedError(ctx, r.Status(), r.Recorder, topic, topic.Generation, err,
			syncStatusOpts{mqObjectExists: &mqExists})
	}
	if driftMsg != "" {
		metrics.RecordDriftDetected(metrics.ControllerTopic)
		opts := syncStatusOpts{mqObjectExists: boolPtr(mqExists)}
		if patchErr := patchSyncedDrift(
			ctx, r.Status(), r.Recorder, topic, topic.Generation, driftMsg, opts,
		); patchErr != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", patchErr)
		}
		return workloadDriftResyncResult(), nil
	}

	if err := patchSyncedAvailable(ctx, r.Status(), r.Recorder, topic, topic.Generation,
		"Topic matches spec", syncStatusOpts{mqObjectExists: boolPtr(true)}); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("Topic synced", "topic", topic.Spec.TopicName)
	return workloadDriftResyncResult(), nil
}

func (r *TopicReconciler) ensureTopic(
	ctx context.Context,
	admin mqadmin.Admin,
	topic *messagingv1alpha1.Topic,
	spec mqadmin.TopicSpec,
	observeOnly bool,
) (bool, string, error) {
	mqCtx, cancel := MQRequestContext(ctx)
	defer cancel()

	observed, err := admin.GetTopic(mqCtx, spec.Name)
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
		workloadAdoptionPolicy(topic),
		workloadFirstAdoption(topic),
		exists,
		observedAttrs,
		spec.Attributes,
		mqrest.TopicDriftCheckKeys(),
		fmt.Sprintf("topic %q", spec.Name),
		func() error { return admin.DefineTopic(mqCtx, spec) },
	)
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

	mqCtx, cancel := MQRequestContext(ctx)
	defer cancel()

	if err := admin.DeleteTopic(mqCtx, topic.Spec.TopicName); err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, topic, topic.Generation, err, syncStatusOpts{})
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
