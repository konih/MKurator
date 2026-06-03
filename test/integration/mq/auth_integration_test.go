//go:build integration

package mq

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/konih/kurator/internal/mqadmin"
)

func TestIntegration_GetChannelAuth(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	channel := channelNameForTest(t.Name())

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
		Description: "integration get path",
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

	state, err := c.GetChannelAuth(ctx, authSpec)
	if err != nil {
		t.Fatalf("GetChannelAuth: %v", err)
	}
	if state.Address != "*" || state.UserSource != "CHANNEL" {
		t.Fatalf("state = %+v", state)
	}
}

func TestIntegration_GetChannelAuth_NotFound(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.GetChannelAuth(ctx, mqadmin.ChannelAuthSpec{
		ChannelName: channelNameForTest(t.Name()+".missing"),
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
	})
	if err == nil || !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetAuthority(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	profile := queueNameForTest(t.Name())

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

	state, err := c.GetAuthority(ctx, authSpec)
	if err != nil {
		t.Fatalf("GetAuthority: %v", err)
	}
	if len(state.Authorities) < 2 {
		t.Fatalf("authorities = %v", state.Authorities)
	}
}

func TestIntegration_GetAuthority_NotFound(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.GetAuthority(ctx, mqadmin.AuthoritySpec{
		Profile:    queueNameForTest(t.Name() + ".missing"),
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "nobody",
	})
	if err == nil || !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_DeleteChannelAuth(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	channel := channelNameForTest(t.Name())

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
		Description: "integration delete path",
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
		t.Fatalf("DeleteChannelAuth: %v", err)
	}

	_, err = c.GetChannelAuth(ctx, authSpec)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_DeleteAuthority(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	profile := queueNameForTest(t.Name())

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
		t.Fatalf("DeleteAuthority: %v", err)
	}

	_, err = c.GetAuthority(ctx, authSpec)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_ChannelAuth_UpdateViaReplace(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	channel := channelNameForTest("UPDATE." + t.Name())

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
	base := mqadmin.ChannelAuthSpec{
		ChannelName: channel,
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		UserSource:  "CHANNEL",
	}
	t.Cleanup(func() {
		_ = c.DeleteChannelAuth(context.Background(), base)
		_ = c.DeleteChannel(context.Background(), chSpec)
	})

	if err := c.DefineChannel(ctx, chSpec); err != nil {
		t.Fatalf("DefineChannel: %v", err)
	}

	set := func(descr, checkClient string) {
		t.Helper()
		spec := base
		spec.Description = descr
		spec.CheckClient = checkClient
		if err := c.SetChannelAuth(ctx, spec); err != nil {
			t.Fatalf("SetChannelAuth descr=%s checkClient=%s: %v", descr, checkClient, err)
		}
	}

	set("kurator chlauth v1", "REQUIRED")
	state, err := c.GetChannelAuth(ctx, base)
	if err != nil {
		t.Fatalf("GetChannelAuth: %v", err)
	}
	if state.Description != "kurator chlauth v1" || !strings.EqualFold(state.CheckClient, "REQUIRED") {
		t.Fatalf("state after v1 = %+v", state)
	}

	set("kurator chlauth v2", "ASQADMIN")
	state, err = c.GetChannelAuth(ctx, base)
	if err != nil {
		t.Fatalf("GetChannelAuth after update: %v", err)
	}
	if state.Description != "kurator chlauth v2" || !strings.EqualFold(state.CheckClient, "ASQADMIN") {
		t.Fatalf("state after v2 = %+v", state)
	}
}

func TestIntegration_Authority_UpdateViaReplace(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	profile := queueNameForTest("UPDATE." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	queueSpec := mqadmin.QueueSpec{
		Name:       profile,
		Type:       mqadmin.QueueTypeLocal,
		Attributes: map[string]string{"maxdepth": "100"},
	}
	base := mqadmin.AuthoritySpec{
		Profile:    profile,
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	}
	t.Cleanup(func() {
		_ = c.DeleteAuthority(context.Background(), base)
		_ = c.DeleteQueue(context.Background(), queueSpec)
	})

	if err := c.DefineQueue(ctx, queueSpec); err != nil {
		t.Fatalf("DefineQueue: %v", err)
	}

	set := func(authorities ...string) {
		t.Helper()
		spec := base
		spec.Authorities = authorities
		if err := c.SetAuthority(ctx, spec); err != nil {
			t.Fatalf("SetAuthority %v: %v", authorities, err)
		}
	}

	set("GET", "PUT")
	state, err := c.GetAuthority(ctx, base)
	if err != nil {
		t.Fatalf("GetAuthority: %v", err)
	}
	if !authoritySetEqual(state.Authorities, []string{"GET", "PUT"}) {
		t.Fatalf("authorities after v1 = %v", state.Authorities)
	}

	set("GET", "INQ")
	state, err = c.GetAuthority(ctx, base)
	if err != nil {
		t.Fatalf("GetAuthority after update: %v", err)
	}
	if !authoritySetEqual(state.Authorities, []string{"GET", "PUT", "INQ"}) {
		t.Fatalf("authorities after v2 = %v", state.Authorities)
	}
}

func TestIntegration_DeleteChannelAuth_Idempotent(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	channel := channelNameForTest("GONE." + t.Name())

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
	}
	t.Cleanup(func() { _ = c.DeleteChannel(context.Background(), chSpec) })

	if err := c.DefineChannel(ctx, chSpec); err != nil {
		t.Fatalf("DefineChannel: %v", err)
	}

	if err := c.DeleteChannelAuth(ctx, authSpec); err != nil {
		t.Fatalf("DeleteChannelAuth first on missing rule: %v", err)
	}
	if err := c.DeleteChannelAuth(ctx, authSpec); err != nil {
		t.Fatalf("DeleteChannelAuth second on missing rule: %v", err)
	}
}

func TestIntegration_DeleteAuthority_Idempotent(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	authSpec := mqadmin.AuthoritySpec{
		Profile:    queueNameForTest("GONE." + t.Name()),
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "nobody",
	}

	if err := c.DeleteAuthority(ctx, authSpec); err != nil {
		t.Fatalf("DeleteAuthority first on missing record: %v", err)
	}
	if err := c.DeleteAuthority(ctx, authSpec); err != nil {
		t.Fatalf("DeleteAuthority second on missing record: %v", err)
	}
}

func authoritySetEqual(got, want []string) bool {
	normalize := func(in []string) []string {
		out := make([]string, 0, len(in))
		for _, auth := range in {
			auth = strings.ToUpper(strings.TrimSpace(auth))
			if auth != "" {
				out = append(out, auth)
			}
		}
		sort.Strings(out)
		return out
	}
	gotNorm := normalize(got)
	wantNorm := normalize(want)
	if len(gotNorm) != len(wantNorm) {
		return false
	}
	for i := range gotNorm {
		if gotNorm[i] != wantNorm[i] {
			return false
		}
	}
	return true
}
