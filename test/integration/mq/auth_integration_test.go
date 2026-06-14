//go:build integration

package mq

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
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

func TestIntegration_GetChannelAuth_BlockAddr(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	address := blockAddrForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	authSpec := mqadmin.ChannelAuthSpec{
		ChannelName: "*",
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockAddr,
		Address:     address,
		Description: "integration blockaddr path",
	}
	t.Cleanup(func() {
		_ = c.DeleteChannelAuth(context.Background(), authSpec)
	})

	if err := c.SetChannelAuth(ctx, authSpec); err != nil {
		t.Fatalf("SetChannelAuth: %v", err)
	}

	state, err := c.GetChannelAuth(ctx, authSpec)
	if err != nil {
		t.Fatalf("GetChannelAuth: %v", err)
	}
	if state.Address != address {
		t.Fatalf("address = %q, want %q", state.Address, address)
	}
}

func TestIntegration_DeleteChannelAuth_BlockAddr(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	address := blockAddrForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	authSpec := mqadmin.ChannelAuthSpec{
		ChannelName: "*",
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockAddr,
		Address:     address,
		Description: "integration blockaddr delete path",
	}
	t.Cleanup(func() {
		_ = c.DeleteChannelAuth(context.Background(), authSpec)
	})

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

func TestIntegration_GetChannelAuth_UserMap(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	channel := channelNameForTest(t.Name())
	clientUser := clientUserForTest(t.Name())

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
		RuleType:    mqadmin.ChannelAuthRuleTypeUserMap,
		ClientUser:  clientUser,
		UserSource:  "MAP",
		McaUser:     "app",
		Description: "integration usermap path",
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
	if !strings.EqualFold(state.ClientUser, clientUser) {
		t.Fatalf("clientUser = %q, want %q", state.ClientUser, clientUser)
	}
	if !strings.EqualFold(state.McaUser, "app") {
		t.Fatalf("mcaUser = %q, want app", state.McaUser)
	}
	if !strings.EqualFold(state.UserSource, "MAP") {
		t.Fatalf("userSource = %q, want MAP", state.UserSource)
	}
	if mqadmin.ChannelAuthNeedsUpdate(authSpec, state) {
		t.Fatalf("ChannelAuthNeedsUpdate after set; state=%+v", state)
	}
}

func TestIntegration_GetChannelAuth_UserMap_UserSourceChannel(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	channel := channelNameForTest(t.Name())
	clientUser := clientUserForTest(t.Name())

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
		RuleType:    mqadmin.ChannelAuthRuleTypeUserMap,
		ClientUser:  clientUser,
		UserSource:  "CHANNEL",
		Description: "integration usermap channel source",
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
	if !strings.EqualFold(state.ClientUser, clientUser) {
		t.Fatalf("clientUser = %q, want %q", state.ClientUser, clientUser)
	}
	if !strings.EqualFold(state.UserSource, "CHANNEL") {
		t.Fatalf("userSource = %q, want CHANNEL", state.UserSource)
	}
	if mqadmin.ChannelAuthNeedsUpdate(authSpec, state) {
		t.Fatalf("ChannelAuthNeedsUpdate after set; state=%+v", state)
	}
}

func TestIntegration_DeleteChannelAuth_UserMap(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	channel := channelNameForTest(t.Name())
	clientUser := clientUserForTest(t.Name())

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
		RuleType:    mqadmin.ChannelAuthRuleTypeUserMap,
		ClientUser:  clientUser,
		UserSource:  "MAP",
		McaUser:     "app",
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

func TestIntegration_GetChannelAuth_BlockUser(t *testing.T) {
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
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockUser,
		UserList:    "nobody",
		Description: "integration blockuser path",
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
	if !strings.EqualFold(state.UserList, "nobody") {
		t.Fatalf("userList = %q", state.UserList)
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

func TestIntegration_GetAuthority_Channel(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	profile := channelNameForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	chSpec := mqadmin.ChannelSpec{
		Name: profile,
		Type: mqadmin.ChannelTypeSvrconn,
		Attributes: map[string]string{
			"trptype": "tcp",
		},
	}
	authSpec := mqadmin.AuthoritySpec{
		Profile:     profile,
		ObjectType:  mqadmin.AuthorityObjectTypeChannel,
		Principal:   "app",
		Authorities: []string{"CHG", "DSP"},
	}
	t.Cleanup(func() {
		_ = c.DeleteAuthority(context.Background(), authSpec)
		_ = c.DeleteChannel(context.Background(), chSpec)
	})

	if err := c.DefineChannel(ctx, chSpec); err != nil {
		t.Fatalf("DefineChannel: %v", err)
	}
	if err := c.SetAuthority(ctx, authSpec); err != nil {
		t.Fatalf("SetAuthority: %v", err)
	}

	state, err := c.GetAuthority(ctx, authSpec)
	if err != nil {
		t.Fatalf("GetAuthority: %v", err)
	}
	if !authoritySetEqual(state.Authorities, []string{"CHG", "DSP"}) {
		t.Fatalf("authorities = %v", state.Authorities)
	}
}

func TestIntegration_GetAuthority_Topic(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	profile := topicNameForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	topstr := fmt.Sprintf("mkurator/it/%d", testNameHash(t.Name())%100000)
	topicSpec := mqadmin.TopicSpec{
		Name: profile,
		Attributes: map[string]string{
			"topstr": topstr,
		},
	}
	authSpec := mqadmin.AuthoritySpec{
		Profile:     profile,
		ObjectType:  mqadmin.AuthorityObjectTypeTopic,
		Principal:   "app",
		Authorities: []string{"SUB", "DSP"},
	}
	t.Cleanup(func() {
		_ = c.DeleteAuthority(context.Background(), authSpec)
		_ = c.DeleteTopic(context.Background(), profile)
	})

	if err := c.DefineTopic(ctx, topicSpec); err != nil {
		t.Fatalf("DefineTopic: %v", err)
	}
	if err := c.SetAuthority(ctx, authSpec); err != nil {
		t.Fatalf("SetAuthority: %v", err)
	}

	state, err := c.GetAuthority(ctx, authSpec)
	if err != nil {
		t.Fatalf("GetAuthority: %v", err)
	}
	if !authoritySetEqual(state.Authorities, []string{"SUB", "DSP"}) {
		t.Fatalf("authorities = %v", state.Authorities)
	}
}

func TestIntegration_GetAuthority_Channel_NotFound(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.GetAuthority(ctx, mqadmin.AuthoritySpec{
		Profile:    channelNameForTest(t.Name() + ".missing"),
		ObjectType: mqadmin.AuthorityObjectTypeChannel,
		Principal:  "nobody",
	})
	if err == nil || !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetAuthority_Topic_NotFound(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.GetAuthority(ctx, mqadmin.AuthoritySpec{
		Profile:    topicNameForTest(t.Name() + ".missing"),
		ObjectType: mqadmin.AuthorityObjectTypeTopic,
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

	state, err := c.GetAuthority(ctx, authSpec)
	if err != nil {
		if errors.Is(err, mqadmin.ErrNotFound) {
			return
		}
		t.Fatalf("GetAuthority: %v", err)
	}
	if !authorityRemoved(state.Authorities) {
		t.Fatalf("expected authority removed after delete, got %v", state.Authorities)
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

	set := func(checkClient string) {
		t.Helper()
		spec := base
		spec.CheckClient = checkClient
		if err := c.SetChannelAuth(ctx, spec); err != nil {
			t.Fatalf("SetChannelAuth checkClient=%s: %v", checkClient, err)
		}
	}

	set("REQUIRED")
	state, err := c.GetChannelAuth(ctx, base)
	if err != nil {
		t.Fatalf("GetChannelAuth: %v", err)
	}
	if !strings.EqualFold(state.CheckClient, "REQUIRED") {
		t.Fatalf("checkClient after v1 = %q", state.CheckClient)
	}

	set("REQDADM")
	state, err = c.GetChannelAuth(ctx, base)
	if err != nil {
		t.Fatalf("GetChannelAuth after update: %v", err)
	}
	if !strings.EqualFold(state.CheckClient, "REQDADM") {
		t.Fatalf("checkClient after v2 = %q", state.CheckClient)
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

func authorityRemoved(authorities []string) bool {
	if len(authorities) == 0 {
		return true
	}
	return len(authorities) == 1 && strings.EqualFold(authorities[0], "NONE")
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
