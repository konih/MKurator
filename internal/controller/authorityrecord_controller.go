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

// AuthorityRecordReconciler reconciles AuthorityRecord objects into OAM on IBM MQ.
type AuthorityRecordReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	MQFactory mqadmin.Factory
	Recorder  events.EventRecorder
}

//nolint:lll // kubebuilder rbac marker is a single line
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=authorityrecords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=authorityrecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=authorityrecords/finalizers,verbs=update
// +kubebuilder:rbac:groups=messaging.kurator.dev,resources=queuemanagerconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile ensures the AUTHREC matches spec.
func (r *AuthorityRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	result, err := r.reconcile(ctx, req)
	metrics.RecordReconcile(metrics.ControllerAuthorityRecord, err)
	return result, err
}

//nolint:dupl // shared MQ object reconcile flow; differs in ensure/delete/spec mapping
func (r *AuthorityRecordReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	auth := &messagingv1alpha1.AuthorityRecord{}
	if err := r.Get(ctx, req.NamespacedName, auth); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get AuthorityRecord: %w", err)
	}

	connRef, err := connectionRefName(auth)
	if err != nil {
		return ctrl.Result{}, err
	}
	conn, err := resolveConnection(ctx, r.Client, auth.Namespace, connRef)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, auth, auth.Generation, err, syncStatusOpts{})
	}

	waitResult, waitDone, waitErr := waitForConnectionReady(
		ctx, r.Status(), r.Recorder, auth, conn, auth.Generation,
	)
	if waitDone {
		return waitResult, waitErr
	}

	admin, err := r.MQFactory.ForConnection(ctx, conn)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, auth, auth.Generation, err, syncStatusOpts{})
	}

	if !auth.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, auth, admin)
	}

	if !controllerutil.ContainsFinalizer(auth, messagingv1alpha1.AuthorityRecordFinalizer) {
		controllerutil.AddFinalizer(auth, messagingv1alpha1.AuthorityRecordFinalizer)
		if updateErr := r.Update(ctx, auth); updateErr != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", updateErr)
		}
		return ctrl.Result{}, nil
	}

	spec := toMQAuthoritySpec(auth)
	desiredMQSC, formatErr := mqrest.FormatSetAuthorityMQSC(spec)
	if formatErr != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, auth, auth.Generation, formatErr, syncStatusOpts{})
	}
	auth.Status.DesiredMQSC = desiredMQSC

	mqExists, drifted, err := r.ensureAuthority(ctx, admin, spec, auth)
	if err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, auth, auth.Generation, err,
			syncStatusOpts{mqObjectExists: &mqExists})
	}
	if drifted {
		msg := "AUTHREC on IBM MQ differs from spec (observe-only; not applying)"
		if err := patchSyncedDrift(ctx, r.Status(), r.Recorder, auth, auth.Generation, msg,
			syncStatusOpts{mqObjectExists: boolPtr(true)}); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", err)
		}
		logger.Info("AuthorityRecord drift detected (observe-only)", "profile", auth.Spec.Profile)
		return ctrl.Result{}, nil
	}

	if err := patchSyncedAvailable(ctx, r.Status(), r.Recorder, auth, auth.Generation,
		"AuthorityRecord matches spec", syncStatusOpts{mqObjectExists: boolPtr(true)}); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	logger.Info("AuthorityRecord synced", "profile", auth.Spec.Profile, "type", auth.Spec.ObjectType)
	return ctrl.Result{}, nil
}

func (r *AuthorityRecordReconciler) ensureAuthority(
	ctx context.Context,
	admin mqadmin.Admin,
	spec mqadmin.AuthoritySpec,
	auth *messagingv1alpha1.AuthorityRecord,
) (bool, bool, error) {
	observed, err := admin.GetAuthority(ctx, spec)
	if err != nil && !errors.Is(err, mqadmin.ErrNotFound) {
		return false, false, err
	}
	exists := observed != nil
	if observed == nil || mqadmin.AuthorityNeedsUpdate(spec, observed) {
		if observed != nil && isObserveOnly(auth) {
			return true, true, nil
		}
		if err := admin.SetAuthority(ctx, spec); err != nil {
			return exists, false, err
		}
		return true, false, nil
	}
	return true, false, nil
}

func (r *AuthorityRecordReconciler) handleDeletion(
	ctx context.Context,
	auth *messagingv1alpha1.AuthorityRecord,
	admin mqadmin.Admin,
) (ctrl.Result, error) {
	if err := patchSyncedDeleting(ctx, r.Status(), r.Recorder, auth, auth.Generation,
		"Deleting authority record from IBM MQ"); err != nil {
		return ctrl.Result{}, err
	}

	spec := toMQAuthoritySpec(auth)
	if err := admin.DeleteAuthority(ctx, spec); err != nil {
		return setSyncedError(ctx, r.Status(), r.Recorder, auth, auth.Generation, err, syncStatusOpts{})
	}

	recordNormalEvent(r.Recorder, auth, EventReasonDeleted, "Authority record removed from IBM MQ")

	controllerutil.RemoveFinalizer(auth, messagingv1alpha1.AuthorityRecordFinalizer)
	if err := r.Update(ctx, auth); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

func toMQAuthoritySpec(auth *messagingv1alpha1.AuthorityRecord) mqadmin.AuthoritySpec {
	authorities := append([]string(nil), auth.Spec.Authorities...)
	return mqadmin.AuthoritySpec{
		Profile:     auth.Spec.Profile,
		ObjectType:  mqadmin.AuthorityObjectType(auth.Spec.ObjectType),
		Principal:   auth.Spec.Principal,
		Group:       auth.Spec.Group,
		Authorities: authorities,
	}
}

// SetupWithManager wires the reconciler.
func (r *AuthorityRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return setupMQObjectController(mgr, r, &messagingv1alpha1.AuthorityRecord{})
}
