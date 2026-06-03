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

// ChannelAuthRuleReconciler reconciles ChannelAuthRule objects into CHLAUTH on IBM MQ.
type ChannelAuthRuleReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
	Recorder  events.EventRecorder
}

//nolint:lll // kubebuilder rbac marker is a single line
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=channelauthrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=channelauthrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=channelauthrules/finalizers,verbs=update
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queuemanagerconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile ensures the CHLAUTH rule matches spec.
func (r *ChannelAuthRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	result, err := r.reconcile(ctx, req)
	metrics.RecordReconcile(metrics.ControllerChannelAuthRule, err)
	return result, err
}

//nolint:dupl // shared MQ object reconcile flow; differs in ensure/delete/spec mapping
func (r *ChannelAuthRuleReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	rule := &messagingv1alpha1.ChannelAuthRule{}
	if err := r.Get(ctx, req.NamespacedName, rule); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ChannelAuthRule: %w", err)
	}

	connRef, err := connectionRefName(rule)
	if err != nil {
		return ctrl.Result{}, err
	}
	conn, err := resolveConnection(ctx, r.Client, rule.Namespace, connRef)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, rule, rule.Generation, err, syncStatusOpts{})
	}

	waitResult, waitDone, waitErr := waitForConnectionReady(
		ctx, r.Status(), r.Recorder, rule, conn, rule.Generation,
	)
	if waitDone {
		return waitResult, waitErr
	}

	admin, err := r.MQFactory.ForConnection(ctx, conn)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, rule, rule.Generation, err, syncStatusOpts{})
	}

	if !rule.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, rule, admin)
	}

	if !controllerutil.ContainsFinalizer(rule, messagingv1alpha1.ChannelAuthRuleFinalizer) {
		controllerutil.AddFinalizer(rule, messagingv1alpha1.ChannelAuthRuleFinalizer)
		if updateErr := r.Update(ctx, rule); updateErr != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", updateErr)
		}
		return ctrl.Result{}, nil
	}

	spec := toMQChannelAuthSpec(rule)
	desiredMQSC, formatErr := mqrest.FormatSetChannelAuthMQSC(spec)
	if formatErr != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, rule, rule.Generation, formatErr, syncStatusOpts{})
	}
	rule.Status.DesiredMQSC = desiredMQSC

	mqExists, drifted, err := r.ensureChannelAuth(ctx, admin, spec, rule)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, rule, rule.Generation, err,
			syncStatusOpts{mqObjectExists: &mqExists})
	}
	if drifted {
		msg := "CHLAUTH on IBM MQ differs from spec (observe-only; not applying)"
		if err := patchSyncedDrift(ctx, r.Status(), r.Recorder, rule, rule.Generation, msg,
			syncStatusOpts{mqObjectExists: boolPtr(true)}); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", err)
		}
		logger.Info("ChannelAuthRule drift detected (observe-only)", "channel", rule.Spec.ChannelName)
		return ctrl.Result{}, nil
	}

	if err := patchSyncedAvailable(ctx, r.Status(), r.Recorder, rule, rule.Generation,
		"ChannelAuthRule matches spec", syncStatusOpts{mqObjectExists: boolPtr(true)}); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("ChannelAuthRule synced", "channel", rule.Spec.ChannelName, "type", rule.Spec.RuleType)
	return ctrl.Result{}, nil
}

func (r *ChannelAuthRuleReconciler) ensureChannelAuth(
	ctx context.Context,
	admin mqadmin.Admin,
	spec mqadmin.ChannelAuthSpec,
	rule *messagingv1alpha1.ChannelAuthRule,
) (bool, bool, error) {
	observed, err := admin.GetChannelAuth(ctx, spec)
	if err != nil && !errors.Is(err, mqadmin.ErrNotFound) {
		return false, false, err
	}
	exists := observed != nil
	if observed == nil || mqadmin.ChannelAuthNeedsUpdate(spec, observed) {
		if observed != nil && isObserveOnly(rule) {
			return true, true, nil
		}
		if err := admin.SetChannelAuth(ctx, spec); err != nil {
			return exists, false, err
		}
		return true, false, nil
	}
	return true, false, nil
}

func (r *ChannelAuthRuleReconciler) handleDeletion(
	ctx context.Context,
	rule *messagingv1alpha1.ChannelAuthRule,
	admin mqadmin.Admin,
) (ctrl.Result, error) {
	if err := patchSyncedDeleting(ctx, r.Status(), r.Recorder, rule, rule.Generation,
		"Deleting CHLAUTH rule from IBM MQ"); err != nil {
		return ctrl.Result{}, err
	}

	spec := toMQChannelAuthSpec(rule)
	if err := admin.DeleteChannelAuth(ctx, spec); err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, rule, rule.Generation, err, syncStatusOpts{})
	}

	recordNormalEvent(r.Recorder, rule, EventReasonDeleted, "CHLAUTH rule removed from IBM MQ")

	controllerutil.RemoveFinalizer(rule, messagingv1alpha1.ChannelAuthRuleFinalizer)
	if err := r.Update(ctx, rule); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

func toMQChannelAuthSpec(rule *messagingv1alpha1.ChannelAuthRule) mqadmin.ChannelAuthSpec {
	return mqadmin.ChannelAuthSpec{
		ChannelName: rule.Spec.ChannelName,
		RuleType:    mqadmin.ChannelAuthRuleType(rule.Spec.RuleType),
		Address:     rule.Spec.Address,
		UserList:    rule.Spec.UserList,
		UserSource:  rule.Spec.UserSource,
		CheckClient: rule.Spec.CheckClient,
		Description: rule.Spec.Description,
	}
}

// SetupWithManager wires the reconciler.
func (r *ChannelAuthRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return setupMQObjectController(mgr, r, &messagingv1alpha1.ChannelAuthRule{})
}
