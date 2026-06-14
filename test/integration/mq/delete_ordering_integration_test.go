//go:build integration

package mq

import (
	"context"
	"errors"
	"testing"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

// Delete-ordering tests document mqweb cleanup semantics for finalizers: both
// orderings should succeed without terminal MQSC errors (idempotent deletes).

func TestIntegration_DeleteOrdering_QueueBeforeAuthority(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	profile := queueNameForTest("ORD.QBEF." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	queueSpec := mqadmin.QueueSpec{
		Name:       profile,
		Type:       mqadmin.QueueTypeLocal,
		Attributes: map[string]string{"maxdepth": "100"},
	}
	authSpec := mqadmin.AuthoritySpec{
		Profile:     profile,
		ObjectType:  mqadmin.AuthorityObjectTypeQueue,
		Principal:   "app",
		Authorities: []string{"GET", "PUT"},
	}
	t.Cleanup(func() {
		_ = c.DeleteAuthority(context.Background(), authSpec)
		_ = c.DeleteQueue(context.Background(), queueSpec)
	})

	if err := c.DefineQueue(ctx, queueSpec); err != nil {
		t.Fatalf("DefineQueue: %v", err)
	}
	if err := c.SetAuthority(ctx, authSpec); err != nil {
		t.Fatalf("SetAuthority: %v", err)
	}

	if err := c.DeleteQueue(ctx, queueSpec); err != nil {
		t.Fatalf("DeleteQueue before authority: %v", err)
	}
	if err := c.DeleteAuthority(ctx, authSpec); err != nil {
		t.Fatalf("DeleteAuthority after queue removed: %v", err)
	}

	assertQueueGone(t, ctx, c, queueSpec)
	assertAuthorityGone(t, ctx, c, authSpec)
}

func TestIntegration_DeleteOrdering_AuthorityBeforeQueue(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	profile := queueNameForTest("ORD.ABEF." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	queueSpec := mqadmin.QueueSpec{
		Name:       profile,
		Type:       mqadmin.QueueTypeLocal,
		Attributes: map[string]string{"maxdepth": "100"},
	}
	authSpec := mqadmin.AuthoritySpec{
		Profile:     profile,
		ObjectType:  mqadmin.AuthorityObjectTypeQueue,
		Principal:   "app",
		Authorities: []string{"GET", "PUT"},
	}
	t.Cleanup(func() {
		_ = c.DeleteAuthority(context.Background(), authSpec)
		_ = c.DeleteQueue(context.Background(), queueSpec)
	})

	if err := c.DefineQueue(ctx, queueSpec); err != nil {
		t.Fatalf("DefineQueue: %v", err)
	}
	if err := c.SetAuthority(ctx, authSpec); err != nil {
		t.Fatalf("SetAuthority: %v", err)
	}

	if err := c.DeleteAuthority(ctx, authSpec); err != nil {
		t.Fatalf("DeleteAuthority before queue: %v", err)
	}
	if err := c.DeleteQueue(ctx, queueSpec); err != nil {
		t.Fatalf("DeleteQueue after authority removed: %v", err)
	}

	assertQueueGone(t, ctx, c, queueSpec)
	assertAuthorityGone(t, ctx, c, authSpec)
}

func TestIntegration_DeleteOrdering_ChannelAuthBeforeChannel(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	channel := channelNameForTest("ORD.CABEF." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	chSpec := mqadmin.ChannelSpec{
		Name: channel,
		Type: mqadmin.ChannelTypeSvrconn,
		Attributes: map[string]string{
			"trptype": "tcp",
		},
	}
	authSpec := mqadmin.ChannelAuthSpec{
		ChannelName: channel,
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "REQUIRED",
	}
	t.Cleanup(func() {
		_ = c.DeleteChannelAuth(context.Background(), authSpec)
		_ = c.DeleteChannel(context.Background(), chSpec)
	})

	if err := c.DefineChannel(ctx, chSpec); err != nil {
		t.Fatalf("DefineChannel: %v", err)
	}
	if err := c.SetChannelAuth(ctx, authSpec); err != nil {
		t.Fatalf("SetChannelAuth: %v", err)
	}

	if err := c.DeleteChannelAuth(ctx, authSpec); err != nil {
		t.Fatalf("DeleteChannelAuth before channel: %v", err)
	}
	if err := c.DeleteChannel(ctx, chSpec); err != nil {
		t.Fatalf("DeleteChannel after CHLAUTH removed: %v", err)
	}

	assertChannelGone(t, ctx, c, chSpec)
	assertChannelAuthGone(t, ctx, c, authSpec)
}

func TestIntegration_DeleteOrdering_ChannelBeforeChannelAuth(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	channel := channelNameForTest("ORD.CBEF." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	chSpec := mqadmin.ChannelSpec{
		Name: channel,
		Type: mqadmin.ChannelTypeSvrconn,
		Attributes: map[string]string{
			"trptype": "tcp",
		},
	}
	authSpec := mqadmin.ChannelAuthSpec{
		ChannelName: channel,
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "REQUIRED",
	}
	t.Cleanup(func() {
		_ = c.DeleteChannelAuth(context.Background(), authSpec)
		_ = c.DeleteChannel(context.Background(), chSpec)
	})

	if err := c.DefineChannel(ctx, chSpec); err != nil {
		t.Fatalf("DefineChannel: %v", err)
	}
	if err := c.SetChannelAuth(ctx, authSpec); err != nil {
		t.Fatalf("SetChannelAuth: %v", err)
	}

	if err := c.DeleteChannel(ctx, chSpec); err != nil {
		t.Fatalf("DeleteChannel before CHLAUTH: %v", err)
	}
	if err := c.DeleteChannelAuth(ctx, authSpec); err != nil {
		t.Fatalf("DeleteChannelAuth after channel removed: %v", err)
	}

	assertChannelGone(t, ctx, c, chSpec)
	assertChannelAuthGone(t, ctx, c, authSpec)
}

func assertQueueGone(t *testing.T, ctx context.Context, c mqadmin.Admin, spec mqadmin.QueueSpec) {
	t.Helper()
	_, err := c.GetQueue(ctx, spec)
	if err == nil {
		t.Fatal("expected queue not found after delete ordering")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("GetQueue after delete ordering: %v", err)
	}
}

func assertAuthorityGone(t *testing.T, ctx context.Context, c mqadmin.Admin, spec mqadmin.AuthoritySpec) {
	t.Helper()
	state, err := c.GetAuthority(ctx, spec)
	if err != nil {
		if errors.Is(err, mqadmin.ErrNotFound) {
			return
		}
		t.Fatalf("GetAuthority after delete ordering: %v", err)
	}
	if !authorityRemoved(state.Authorities) {
		t.Fatalf("expected authority removed after delete ordering, got %v", state.Authorities)
	}
}

func assertChannelGone(t *testing.T, ctx context.Context, c mqadmin.Admin, spec mqadmin.ChannelSpec) {
	t.Helper()
	_, err := c.GetChannel(ctx, spec)
	if err == nil {
		t.Fatal("expected channel not found after delete ordering")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("GetChannel after delete ordering: %v", err)
	}
}

func assertChannelAuthGone(t *testing.T, ctx context.Context, c mqadmin.Admin, spec mqadmin.ChannelAuthSpec) {
	t.Helper()
	_, err := c.GetChannelAuth(ctx, spec)
	if err == nil {
		t.Fatal("expected CHLAUTH not found after delete ordering")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("GetChannelAuth after delete ordering: %v", err)
	}
}
