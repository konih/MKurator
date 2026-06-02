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

// ChannelReconciler reconciles Channel objects into MQSC on IBM MQ.
type ChannelReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
	Recorder  record.EventRecorder
}

// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=channels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=channels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=channels/finalizers,verbs=update
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queuemanagerconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile ensures the MQ channel matches spec.
func (r *ChannelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	result, err := r.reconcile(ctx, req)
	metrics.RecordReconcile(metrics.ControllerChannel, err)
	return result, err
}

//nolint:dupl // shared MQ object reconcile flow; differs in ensure/delete/spec mapping
func (r *ChannelReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	channel := &messagingv1alpha1.Channel{}
	if err := r.Get(ctx, req.NamespacedName, channel); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get Channel: %w", err)
	}

	connRef, err := connectionRefName(channel)
	if err != nil {
		return ctrl.Result{}, err
	}
	conn, err := resolveConnection(ctx, r.Client, channel.Namespace, connRef)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, channel, channel.Generation, err)
	}

	waitResult, waitDone, waitErr := waitForConnectionReady(
		ctx,
		r.Status(),
		r.Recorder,
		channel,
		conn,
		channel.Generation,
	)
	if waitDone {
		return waitResult, waitErr
	}

	admin, err := r.MQFactory.ForConnection(ctx, conn)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, channel, channel.Generation, err)
	}

	if !channel.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, channel, admin)
	}

	if !controllerutil.ContainsFinalizer(channel, messagingv1alpha1.ChannelFinalizer) {
		controllerutil.AddFinalizer(channel, messagingv1alpha1.ChannelFinalizer)
		if err := r.Update(ctx, channel); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	if channel.Spec.Type != "" && channel.Spec.Type != messagingv1alpha1.ChannelTypeSvrconn {
		return setSyncedError(ctx, r.Status(), r.Recorder, channel, channel.Generation, &mqadmin.TerminalError{
			Reason:  "UnsupportedChannelType",
			Message: fmt.Sprintf("channel type %q is not supported in v1alpha1", channel.Spec.Type),
		})
	}

	spec := toMQChannelSpec(channel)
	if err := r.ensureChannel(ctx, admin, spec); err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, channel, channel.Generation, err)
	}

	if err := patchSyncedAvailable(ctx, r.Status(), r.Recorder, channel, channel.Generation,
		"Channel matches spec"); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("Channel synced", "channel", channel.Spec.ChannelName, "type", spec.Type)
	return ctrl.Result{}, nil
}

func (r *ChannelReconciler) ensureChannel(ctx context.Context, admin mqadmin.Admin, spec mqadmin.ChannelSpec) error {
	observed, err := admin.GetChannel(ctx, spec)
	if err != nil && !errors.Is(err, mqadmin.ErrNotFound) {
		return err
	}
	if observed == nil || channelNeedsUpdate(spec, observed) {
		if err := admin.DefineChannel(ctx, spec); err != nil {
			return err
		}
	}
	return nil
}

func channelNeedsUpdate(desired mqadmin.ChannelSpec, observed *mqadmin.ChannelState) bool {
	return mqadmin.AttributesNeedUpdate(desired.Attributes, observed.Attributes)
}

func (r *ChannelReconciler) handleDeletion(
	ctx context.Context,
	channel *messagingv1alpha1.Channel,
	admin mqadmin.Admin,
) (ctrl.Result, error) {
	if err := patchSyncedDeleting(ctx, r.Status(), r.Recorder, channel, channel.Generation,
		"Deleting channel from IBM MQ"); err != nil {
		return ctrl.Result{}, err
	}

	spec := toMQChannelSpec(channel)
	if err := admin.DeleteChannel(ctx, spec); err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, channel, channel.Generation, err)
	}

	recordNormalEvent(r.Recorder, channel, EventReasonDeleted, "Channel removed from IBM MQ")

	controllerutil.RemoveFinalizer(channel, messagingv1alpha1.ChannelFinalizer)
	if err := r.Update(ctx, channel); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

func toMQChannelSpec(channel *messagingv1alpha1.Channel) mqadmin.ChannelSpec {
	attrs := map[string]string{}
	for k, v := range channel.Spec.Attributes {
		attrs[mqadmin.NormalizeAttrKey(k)] = v
	}
	chType := mqadmin.ChannelTypeSvrconn
	if channel.Spec.Type != "" {
		chType = mqadmin.ChannelType(channel.Spec.Type)
	}
	return mqadmin.ChannelSpec{
		Name:       channel.Spec.ChannelName,
		Type:       chType,
		Attributes: attrs,
	}
}

// SetupWithManager wires the reconciler.
func (r *ChannelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return setupMQObjectController(mgr, r, &messagingv1alpha1.Channel{})
}
