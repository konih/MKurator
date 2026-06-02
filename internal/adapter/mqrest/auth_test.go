package mqrest

import (
	"errors"
	"strings"
	"testing"

	"github.com/konih/kurator/internal/mqadmin"
)

func TestBuildSetChannelAuthMQSC(t *testing.T) {
	cmd, err := buildSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP.SVRCONN.0TLS",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "REQUIRED",
		Description: "Allows connection via APP channel",
	}, "REPLACE")
	if err != nil {
		t.Fatalf("buildSetChannelAuthMQSC: %v", err)
	}
	want := "SET CHLAUTH('DEV.APP.SVRCONN.0TLS') TYPE(ADDRESSMAP) ADDRESS('*') " +
		"USERSRC(CHANNEL) CHCKCLNT(REQUIRED) DESCR('Allows connection via APP channel') ACTION(REPLACE)"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestBuildSetAuthorityMQSC(t *testing.T) {
	cmd, err := buildSetAuthorityMQSC(mqadmin.AuthoritySpec{
		Profile:     "APP.ORDERS",
		ObjectType:  mqadmin.AuthorityObjectTypeQueue,
		Principal:   "app",
		Authorities: []string{"GET", "PUT"},
	}, false)
	if err != nil {
		t.Fatalf("buildSetAuthorityMQSC: %v", err)
	}
	want := "SET AUTHREC PROFILE('APP.ORDERS') OBJTYPE(QUEUE) PRINCIPAL('app') " +
		"AUTHADD(GET,PUT) ACTION(REPLACE)"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestBuildSetAuthorityMQSCRemove(t *testing.T) {
	cmd, err := buildSetAuthorityMQSC(mqadmin.AuthoritySpec{
		Profile:    "APP.ORDERS",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Group:      "apps",
	}, true)
	if err != nil {
		t.Fatalf("buildSetAuthorityMQSC: %v", err)
	}
	want := "SET AUTHREC PROFILE('APP.ORDERS') OBJTYPE(QUEUE) GROUP('apps') AUTHRMV(ALL) ACTION(REPLACE)"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestBuildSetChannelAuthMQSCValidation(t *testing.T) {
	_, err := buildSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{}, "REPLACE")
	if err == nil {
		t.Fatal("expected error for empty channel name")
	}
	_, err = buildSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{ChannelName: "CH1"}, "REPLACE")
	if err == nil {
		t.Fatal("expected error for empty rule type")
	}
}

func TestBuildSetAuthorityMQSCValidation(t *testing.T) {
	_, err := buildSetAuthorityMQSC(mqadmin.AuthoritySpec{}, false)
	if err == nil {
		t.Fatal("expected error for empty spec")
	}
	_, err = buildSetAuthorityMQSC(mqadmin.AuthoritySpec{
		Profile:    "APP.ORDERS",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	}, false)
	if err == nil {
		t.Fatal("expected error when authorities missing")
	}
	_, err = buildSetAuthorityMQSC(mqadmin.AuthoritySpec{
		Profile:     "APP.ORDERS",
		ObjectType:  mqadmin.AuthorityObjectTypeQueue,
		Principal:   "app",
		Group:       "apps",
		Authorities: []string{"GET"},
	}, false)
	if err == nil {
		t.Fatal("expected error when both principal and group set")
	}
}

func TestIsMQSCNotFound(t *testing.T) {
	if !isMQSCNotFound(errors.New("AMQ8147E: object not found")) {
		t.Fatal("expected not found")
	}
	if !isMQSCNotFound(errors.New("AMQ8958E: not found")) {
		t.Fatal("expected AMQ8958 not found")
	}
	if !isMQSCNotFound(mqadmin.ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
	if isMQSCNotFound(errors.New("other error")) {
		t.Fatal("expected false")
	}
}

func TestMqscQuote(t *testing.T) {
	if got := mqscQuote("it's"); got != "it''s" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildDisplayChannelAuthMQSC(t *testing.T) {
	cmd, err := buildDisplayChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "DISPLAY CHLAUTH('DEV.APP') TYPE(ADDRESSMAP)"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestBuildDisplayChannelAuthMQSCValidation(t *testing.T) {
	_, err := buildDisplayChannelAuthMQSC(mqadmin.ChannelAuthSpec{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildDisplayAuthorityMQSC(t *testing.T) {
	cmd, err := buildDisplayAuthorityMQSC(mqadmin.AuthoritySpec{
		Profile:    "APP.ORDERS",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "DISPLAY AUTHREC PROFILE('APP.ORDERS') OBJTYPE(QUEUE) PRINCIPAL('app')"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestAuthorityStateFromAttributes(t *testing.T) {
	spec := mqadmin.AuthoritySpec{
		Profile:    "APP.ORDERS",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	}
	state := authorityStateFromAttributes(spec, map[string]string{"authlist": "GET, PUT"})
	if len(state.Authorities) != 2 || state.Authorities[0] != "GET" || state.Authorities[1] != "PUT" {
		t.Fatalf("authorities = %v", state.Authorities)
	}
}

func TestBuildDisplayAuthorityMQSCGroup(t *testing.T) {
	cmd, err := buildDisplayAuthorityMQSC(mqadmin.AuthoritySpec{
		Profile:    "APP.ORDERS",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Group:      "apps",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "GROUP('apps')") {
		t.Fatalf("cmd = %q", cmd)
	}
}

func TestBuildDisplayAuthorityMQSCValidation(t *testing.T) {
	_, err := buildDisplayAuthorityMQSC(mqadmin.AuthoritySpec{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChannelAuthStateFromAttributes(t *testing.T) {
	spec := mqadmin.ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
	}
	state := channelAuthStateFromAttributes(spec, map[string]string{
		"address": "*", "usersrc": "CHANNEL", "chckclnt": "REQUIRED", "descr": "d",
	})
	if state.Description != "d" {
		t.Fatalf("state = %+v", state)
	}
}
