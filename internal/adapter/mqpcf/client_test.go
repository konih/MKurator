package mqpcf_test

import (
	"context"
	"strings"
	"testing"

	"github.com/conduit-ops/mkurator/internal/adapter/mqpcf"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

func TestNewClient_RequiresQueueManager(t *testing.T) {
	t.Parallel()
	_, err := mqpcf.NewClient(mqpcf.Config{})
	if err == nil {
		t.Fatal("expected error when queue manager is empty")
	}
}

func TestClient_Ping_NotImplemented(t *testing.T) {
	t.Parallel()
	c, err := mqpcf.NewClient(mqpcf.Config{QueueManager: "QM1"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	err = c.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_AdminStubs_NotImplemented(t *testing.T) {
	t.Parallel()
	c, err := mqpcf.NewClient(mqpcf.Config{QueueManager: "QM1"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := context.Background()
	queueSpec := mqadmin.QueueSpec{Name: "APP.Q", Type: mqadmin.QueueTypeLocal}
	channelSpec := mqadmin.ChannelSpec{Name: "APP.CH", Type: mqadmin.ChannelTypeSvrconn}
	authSpec := mqadmin.ChannelAuthSpec{ChannelName: "APP.CH", RuleType: mqadmin.ChannelAuthRuleTypeAddressMap}
	authoritySpec := mqadmin.AuthoritySpec{
		Profile: "APP.Q", ObjectType: mqadmin.AuthorityObjectTypeQueue, Principal: "app",
		Authorities: []string{"GET"},
	}

	cases := []struct {
		name string
		run  func() error
	}{
		{"GetQueue", func() error { _, err := c.GetQueue(ctx, queueSpec); return err }},
		{"DefineQueue", func() error { return c.DefineQueue(ctx, queueSpec) }},
		{"DeleteQueue", func() error { return c.DeleteQueue(ctx, queueSpec) }},
		{"GetTopic", func() error { _, err := c.GetTopic(ctx, "TOPIC"); return err }},
		{"DefineTopic", func() error {
			return c.DefineTopic(ctx, mqadmin.TopicSpec{Name: "TOPIC"})
		}},
		{"DeleteTopic", func() error { return c.DeleteTopic(ctx, "TOPIC") }},
		{"GetChannel", func() error { _, err := c.GetChannel(ctx, channelSpec); return err }},
		{"DefineChannel", func() error { return c.DefineChannel(ctx, channelSpec) }},
		{"DeleteChannel", func() error { return c.DeleteChannel(ctx, channelSpec) }},
		{"SetChannelAuth", func() error { return c.SetChannelAuth(ctx, authSpec) }},
		{"GetChannelAuth", func() error { _, err := c.GetChannelAuth(ctx, authSpec); return err }},
		{"DeleteChannelAuth", func() error { return c.DeleteChannelAuth(ctx, authSpec) }},
		{"SetAuthority", func() error { return c.SetAuthority(ctx, authoritySpec) }},
		{"GetAuthority", func() error { _, err := c.GetAuthority(ctx, authoritySpec); return err }},
		{"DeleteAuthority", func() error { return c.DeleteAuthority(ctx, authoritySpec) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.run()
			if err == nil {
				t.Fatal("expected not implemented error")
			}
			if !strings.Contains(err.Error(), "not implemented") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
