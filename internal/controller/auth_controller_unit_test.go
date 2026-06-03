package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
	"github.com/konih/kurator/internal/mqadmin"
	mqadmintest "github.com/konih/kurator/test/mocks/mqadmin"
)

func TestChannelAuthRuleReconciler_TransientError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "dev-app-addressmap"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev-app-addressmap",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.ChannelAuthRuleFinalizer},
		},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "DEV.APP.SVRCONN.0TLS",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
			UserSource:    "CHANNEL",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(rule, conn).
		WithObjects(conn, rule).
		Build()

	spec := toMQChannelAuthSpec(rule)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannelAuth(mock.Anything, spec).Return(nil, mqadmin.ErrNotFound)
	mockAdmin.EXPECT().SetChannelAuth(mock.Anything, spec).Return(&mqadmin.TransientError{Message: "timeout"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelAuthRuleReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if !errors.Is(err, mqadmin.ErrTransient) {
		t.Fatalf("expected transient error, got result=%+v err=%v", result, err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("RequeueAfter = %v", result.RequeueAfter)
	}
}

func TestChannelAuthRuleReconciler_DeleteTerminalError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "dev-app-addressmap"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	now := metav1.Now()
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "dev-app-addressmap",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.ChannelAuthRuleFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "DEV.APP.SVRCONN.0TLS",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(rule, conn).
		WithObjects(conn, rule).
		Build()

	spec := toMQChannelAuthSpec(rule)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteChannelAuth(mock.Anything, spec).
		Return(&mqadmin.TerminalError{Message: "delete denied"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelAuthRuleReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var updated messagingv1alpha1.ChannelAuthRule
	if err := cl.Get(ctx, key, &updated); err != nil {
		t.Fatalf("Get after delete error: %v", err)
	}
	if len(updated.Finalizers) == 0 {
		t.Fatal("finalizer should remain when delete fails")
	}
}

func TestChannelAuthRuleReconciler_DeleteSuccessRemovesFinalizer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "dev-app-addressmap"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	now := metav1.Now()
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "dev-app-addressmap",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.ChannelAuthRuleFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "DEV.APP.SVRCONN.0TLS",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(rule, conn).
		WithObjects(conn, rule).
		Build()

	spec := toMQChannelAuthSpec(rule)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteChannelAuth(mock.Anything, spec).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelAuthRuleReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var updated messagingv1alpha1.ChannelAuthRule
	err = cl.Get(ctx, key, &updated)
	if err == nil {
		if len(updated.Finalizers) != 0 {
			t.Fatalf("finalizer should be removed, got %v", updated.Finalizers)
		}
		return
	}
	if !k8serrors.IsNotFound(err) {
		t.Fatalf("Get after delete success: %v", err)
	}
}

func TestChannelAuthRuleReconciler_AddsFinalizer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "dev-app-addressmap"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{Name: "dev-app-addressmap", Namespace: ns},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "DEV.APP.SVRCONN.0TLS",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(rule, conn).
		WithObjects(conn, rule).
		Build()

	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelAuthRuleReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("result = %+v", result)
	}

	updated := &messagingv1alpha1.ChannelAuthRule{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Finalizers) != 1 {
		t.Fatalf("finalizers = %v", updated.Finalizers)
	}
}

func TestAuthorityRecordReconciler_TransientError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "app-orders-get-put"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "app-orders-get-put",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.AuthorityRecordFinalizer},
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth, conn).
		WithObjects(conn, auth).
		Build()

	spec := toMQAuthoritySpec(auth)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetAuthority(mock.Anything, spec).Return(nil, mqadmin.ErrNotFound)
	mockAdmin.EXPECT().SetAuthority(mock.Anything, spec).Return(&mqadmin.TransientError{Message: "timeout"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &AuthorityRecordReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	result, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if !errors.Is(err, mqadmin.ErrTransient) {
		t.Fatalf("expected transient error, got result=%+v err=%v", result, err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("RequeueAfter = %v", result.RequeueAfter)
	}
}

func TestAuthorityRecordReconciler_DeleteTerminalError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "app-orders-get-put"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	now := metav1.Now()
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "app-orders-get-put",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.AuthorityRecordFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth, conn).
		WithObjects(conn, auth).
		Build()

	spec := toMQAuthoritySpec(auth)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteAuthority(mock.Anything, spec).
		Return(&mqadmin.TerminalError{Message: "delete denied"})

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &AuthorityRecordReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var updated messagingv1alpha1.AuthorityRecord
	if err := cl.Get(ctx, key, &updated); err != nil {
		t.Fatalf("Get after delete error: %v", err)
	}
	if len(updated.Finalizers) == 0 {
		t.Fatal("finalizer should remain when delete fails")
	}
}

func TestAuthorityRecordReconciler_DeleteSuccessRemovesFinalizer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "app-orders-get-put"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	now := metav1.Now()
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "app-orders-get-put",
			Namespace:         ns,
			Finalizers:        []string{messagingv1alpha1.AuthorityRecordFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth, conn).
		WithObjects(conn, auth).
		Build()

	spec := toMQAuthoritySpec(auth)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().DeleteAuthority(mock.Anything, spec).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &AuthorityRecordReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var updated messagingv1alpha1.AuthorityRecord
	err = cl.Get(ctx, key, &updated)
	if err == nil {
		if len(updated.Finalizers) != 0 {
			t.Fatalf("finalizer should be removed, got %v", updated.Finalizers)
		}
		return
	}
	if !k8serrors.IsNotFound(err) {
		t.Fatalf("Get after delete success: %v", err)
	}
}

func TestSetSyncedError_TransientChannelAuthRule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	s := unitSchemeOrFatal(t)
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{Name: "car1", Namespace: ns, Generation: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(rule).WithObjects(rule).Build()
	result, err := setSyncedError(
		ctx, cl.Status(), nil, rule, 1, &mqadmin.TransientError{Message: "timeout"}, syncStatusOpts{},
	)
	if !errors.Is(err, mqadmin.ErrTransient) || result.RequeueAfter != 30*time.Second {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestChannelAuthRuleReconciler_NoDriftSkipsSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "dev-app-addressmap"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev-app-addressmap",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.ChannelAuthRuleFinalizer},
		},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "DEV.APP.SVRCONN.0TLS",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
			UserSource:    "CHANNEL",
			CheckClient:   "REQUIRED",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(rule, conn).
		WithObjects(conn, rule).
		Build()

	spec := toMQChannelAuthSpec(rule)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannelAuth(mock.Anything, spec).Return(&mqadmin.ChannelAuthState{
		ChannelName: spec.ChannelName,
		RuleType:    spec.RuleType,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "REQUIRED",
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelAuthRuleReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestChannelAuthRuleReconciler_SetsDesiredMQSCInStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "dev-app-addressmap"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev-app-addressmap",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.ChannelAuthRuleFinalizer},
		},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "DEV.APP.SVRCONN.0TLS",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
			UserSource:    "CHANNEL",
			CheckClient:   "REQUIRED",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(rule, conn).
		WithObjects(conn, rule).
		Build()

	spec := toMQChannelAuthSpec(rule)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannelAuth(mock.Anything, spec).Return(&mqadmin.ChannelAuthState{
		ChannelName: spec.ChannelName,
		RuleType:    spec.RuleType,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "REQUIRED",
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelAuthRuleReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.ChannelAuthRule{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	want := "SET CHLAUTH('DEV.APP.SVRCONN.0TLS') TYPE(ADDRESSMAP) ADDRESS('*') " +
		"USERSRC(CHANNEL) CHCKCLNT(REQUIRED) ACTION(REPLACE)"
	if updated.Status.DesiredMQSC != want {
		t.Fatalf("DesiredMQSC = %q, want %q", updated.Status.DesiredMQSC, want)
	}
}

func TestChannelAuthRuleReconciler_DriftAppliesSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "dev-app-addressmap"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev-app-addressmap",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.ChannelAuthRuleFinalizer},
		},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "DEV.APP.SVRCONN.0TLS",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
			UserSource:    "CHANNEL",
			CheckClient:   "REQUIRED",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(rule, conn).
		WithObjects(conn, rule).
		Build()

	spec := toMQChannelAuthSpec(rule)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannelAuth(mock.Anything, spec).Return(&mqadmin.ChannelAuthState{
		ChannelName: spec.ChannelName,
		RuleType:    spec.RuleType,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "ASQMGR",
	}, nil)
	mockAdmin.EXPECT().SetChannelAuth(mock.Anything, spec).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelAuthRuleReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestChannelAuthRuleReconciler_ObserveOnlyDriftSkipsSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "dev-app-addressmap"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "dev-app-addressmap",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.ChannelAuthRuleFinalizer},
			Annotations: map[string]string{
				messagingv1alpha1.DriftPolicyAnnotation: messagingv1alpha1.DriftPolicyObserveOnly,
			},
		},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "DEV.APP.SVRCONN.0TLS",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
			UserSource:    "CHANNEL",
			CheckClient:   "REQUIRED",
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(rule, conn).
		WithObjects(conn, rule).
		Build()

	spec := toMQChannelAuthSpec(rule)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetChannelAuth(mock.Anything, spec).Return(&mqadmin.ChannelAuthState{
		ChannelName: spec.ChannelName,
		RuleType:    spec.RuleType,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "ASQMGR",
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &ChannelAuthRuleReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.ChannelAuthRule{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	if conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced) !=
		messagingv1alpha1.ReasonDriftDetected {
		t.Fatalf("reason = %q", conditionReason(updated.Status.Conditions, messagingv1alpha1.ConditionSynced))
	}
}

func TestAuthorityRecordReconciler_NoDriftSkipsSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "app-orders-get-put"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "app-orders-get-put",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.AuthorityRecordFinalizer},
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth, conn).
		WithObjects(conn, auth).
		Build()

	spec := toMQAuthoritySpec(auth)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetAuthority(mock.Anything, spec).Return(&mqadmin.AuthorityState{
		Profile:     spec.Profile,
		ObjectType:  spec.ObjectType,
		Principal:   spec.Principal,
		Authorities: []string{"GET", "PUT"},
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &AuthorityRecordReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func TestAuthorityRecordReconciler_SetsDesiredMQSCInStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "app-orders-get-put"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "app-orders-get-put",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.AuthorityRecordFinalizer},
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth, conn).
		WithObjects(conn, auth).
		Build()

	spec := toMQAuthoritySpec(auth)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetAuthority(mock.Anything, spec).Return(&mqadmin.AuthorityState{
		Profile:     spec.Profile,
		ObjectType:  spec.ObjectType,
		Principal:   spec.Principal,
		Authorities: []string{"GET", "PUT"},
	}, nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &AuthorityRecordReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &messagingv1alpha1.AuthorityRecord{}
	if err := cl.Get(ctx, key, updated); err != nil {
		t.Fatal(err)
	}
	want := "SET AUTHREC PROFILE('APP.ORDERS') OBJTYPE(QUEUE) PRINCIPAL('app') AUTHADD(GET,PUT)"
	if updated.Status.DesiredMQSC != want {
		t.Fatalf("DesiredMQSC = %q, want %q", updated.Status.DesiredMQSC, want)
	}
}

func TestAuthorityRecordReconciler_NotFoundCreates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := "kurator-system"
	key := types.NamespacedName{Namespace: ns, Name: "app-orders-get-put"}
	s := unitSchemeOrFatal(t)

	conn := readyConnForUnit(ns)
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "app-orders-get-put",
			Namespace:  ns,
			Finalizers: []string{messagingv1alpha1.AuthorityRecordFinalizer},
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(auth, conn).
		WithObjects(conn, auth).
		Build()

	spec := toMQAuthoritySpec(auth)
	mockAdmin := mqadmintest.NewMockAdmin(t)
	mockAdmin.EXPECT().GetAuthority(mock.Anything, spec).Return(nil, mqadmin.ErrNotFound)
	mockAdmin.EXPECT().SetAuthority(mock.Anything, spec).Return(nil)

	mockFactory := mqadmintest.NewMockFactory(t)
	mockFactory.EXPECT().ForConnection(mock.Anything, mock.Anything).Return(mockAdmin, nil)

	rec := &AuthorityRecordReconciler{Client: cl, Scheme: s, MQFactory: mockFactory}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}
