//go:build integration

package mq

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

func TestIntegration_Topic_CreateGetDelete(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := topicNameForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.DeleteTopic(context.Background(), name) })

	topstr := fmt.Sprintf("mkurator/it/%d", testNameHash(t.Name())%100000)
	spec := mqadmin.TopicSpec{
		Name: name,
		Attributes: map[string]string{
			"topstr": topstr,
		},
	}
	if err := c.DefineTopic(ctx, spec); err != nil {
		t.Fatalf("DefineTopic: %v", err)
	}

	state, err := c.GetTopic(ctx, name)
	if err != nil {
		t.Fatalf("GetTopic: %v", err)
	}
	if state.Name != name {
		t.Fatalf("name = %q", state.Name)
	}
	if state.Attributes["topstr"] != topstr {
		t.Fatalf("topstr = %q, want %q", state.Attributes["topstr"], topstr)
	}

	if err := c.DeleteTopic(ctx, name); err != nil {
		t.Fatalf("DeleteTopic: %v", err)
	}

	_, err = c.GetTopic(ctx, name)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_Topic_UpdateViaReplace(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := topicNameForTest("UPDATE." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.DeleteTopic(context.Background(), name) })

	topstr := fmt.Sprintf("mkurator/it/%d", testNameHash("UPDATE."+t.Name())%100000)

	define := func(descr string) {
		t.Helper()
		err := c.DefineTopic(ctx, mqadmin.TopicSpec{
			Name: name,
			Attributes: map[string]string{
				"topstr": topstr,
				"descr":  descr,
			},
		})
		if err != nil {
			t.Fatalf("DefineTopic descr=%s: %v", descr, err)
		}
	}

	define("mkurator integration v1")
	state, err := c.GetTopic(ctx, name)
	if err != nil {
		t.Fatalf("GetTopic: %v", err)
	}
	if state.Attributes["descr"] != "mkurator integration v1" {
		t.Fatalf("descr after first define = %q", state.Attributes["descr"])
	}

	define("mkurator integration v2")
	state, err = c.GetTopic(ctx, name)
	if err != nil {
		t.Fatalf("GetTopic after update: %v", err)
	}
	if state.Attributes["descr"] != "mkurator integration v2" {
		t.Fatalf("descr after replace = %q", state.Attributes["descr"])
	}
}

func TestIntegration_GetTopic_NotFound(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := topicNameForTest("MISSING." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.GetTopic(ctx, name)
	if err == nil {
		t.Fatal("expected not found")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_DeleteTopic_Idempotent(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := topicNameForTest("GONE." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	if err := c.DeleteTopic(ctx, name); err != nil {
		t.Fatalf("DeleteTopic on missing topic: %v", err)
	}
}

func TestIntegration_Channel_CreateGetDelete(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := channelNameForTest(t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	spec := mqadmin.ChannelSpec{
		Name: name,
		Type: mqadmin.ChannelTypeSvrconn,
		Attributes: map[string]string{
			"trptype": "tcp",
			"descr":   "mkurator integration channel",
		},
	}
	t.Cleanup(func() {
		_ = c.DeleteChannel(context.Background(), mqadmin.ChannelSpec{
			Name: name,
			Type: mqadmin.ChannelTypeSvrconn,
		})
	})

	if err := c.DefineChannel(ctx, spec); err != nil {
		t.Fatalf("DefineChannel: %v", err)
	}

	state, err := c.GetChannel(ctx, spec)
	if err != nil {
		t.Fatalf("GetChannel: %v", err)
	}
	if !strings.EqualFold(state.Attributes["trptype"], "tcp") {
		t.Fatalf("trptype = %q", state.Attributes["trptype"])
	}

	if err := c.DeleteChannel(ctx, spec); err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}

	_, err = c.GetChannel(ctx, spec)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_Channel_UpdateViaReplace(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := channelNameForTest("UPDATE." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}
	base := mqadmin.ChannelSpec{
		Name: name,
		Type: mqadmin.ChannelTypeSvrconn,
	}
	t.Cleanup(func() { _ = c.DeleteChannel(context.Background(), base) })

	define := func(descr string) {
		t.Helper()
		spec := base
		spec.Attributes = map[string]string{
			"trptype": "tcp",
			"descr":   descr,
		}
		if err := c.DefineChannel(ctx, spec); err != nil {
			t.Fatalf("DefineChannel descr=%s: %v", descr, err)
		}
	}

	define("mkurator v1")
	state, err := c.GetChannel(ctx, base)
	if err != nil {
		t.Fatalf("GetChannel: %v", err)
	}
	if state.Attributes["descr"] != "mkurator v1" {
		t.Fatalf("descr after first define = %q", state.Attributes["descr"])
	}

	define("mkurator v2")
	state, err = c.GetChannel(ctx, base)
	if err != nil {
		t.Fatalf("GetChannel after update: %v", err)
	}
	if state.Attributes["descr"] != "mkurator v2" {
		t.Fatalf("descr after replace = %q", state.Attributes["descr"])
	}
}

func TestIntegration_GetChannel_NotFound(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := channelNameForTest("MISSING." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	spec := mqadmin.ChannelSpec{Name: name, Type: mqadmin.ChannelTypeSvrconn}
	_, err = c.GetChannel(ctx, spec)
	if err == nil {
		t.Fatal("expected not found")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_DeleteChannel_Idempotent(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := channelNameForTest("GONE." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	spec := mqadmin.ChannelSpec{Name: name, Type: mqadmin.ChannelTypeSvrconn}
	if err := c.DeleteChannel(ctx, spec); err != nil {
		t.Fatalf("DeleteChannel on missing channel: %v", err)
	}
}

func TestIntegration_DefineChannel_UnsupportedType(t *testing.T) {
	requireIntegration(t)
	ctx := testContext(t)
	name := channelNameForTest("BADTYPE." + t.Name())

	c, err := newIntegrationClient()
	if err != nil {
		t.Fatal(err)
	}

	err = c.DefineChannel(ctx, mqadmin.ChannelSpec{
		Name: name,
		Type: mqadmin.ChannelType("receiver"),
	})
	if !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}
