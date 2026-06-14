package mqrest

import (
	"errors"
	"strings"
	"testing"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
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

func TestBuildSetChannelAuthMQSCBlockAddr(t *testing.T) {
	cmd, err := buildSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "*",
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockAddr,
		Address:     "192.0.2.1",
		Description: "block TEST-NET-1",
	}, "REPLACE")
	if err != nil {
		t.Fatalf("buildSetChannelAuthMQSC: %v", err)
	}
	want := "SET CHLAUTH('*') TYPE(BLOCKADDR) ADDRESS('192.0.2.1') " +
		"DESCR('block TEST-NET-1') ACTION(REPLACE)"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestBuildSetChannelAuthMQSCBlockAddrRemove(t *testing.T) {
	cmd, err := buildSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "*",
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockAddr,
		Address:     "192.0.2.1",
		Description: "ignored on remove",
	}, "REMOVE")
	if err != nil {
		t.Fatalf("buildSetChannelAuthMQSC: %v", err)
	}
	want := "SET CHLAUTH('*') TYPE(BLOCKADDR) ADDRESS('192.0.2.1') ACTION(REMOVE)"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestBuildSetChannelAuthMQSCBlockUser(t *testing.T) {
	cmd, err := buildSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "ORDERS.APP",
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockUser,
		UserList:    "nobody",
		Description: "deny nobody",
	}, "REPLACE")
	if err != nil {
		t.Fatalf("buildSetChannelAuthMQSC: %v", err)
	}
	want := "SET CHLAUTH('ORDERS.APP') TYPE(BLOCKUSER) USERLIST('nobody') " +
		"DESCR('deny nobody') ACTION(REPLACE)"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestBuildSetChannelAuthMQSCDeferredRuleTypes(t *testing.T) {
	// Schema allows USERMAP/SSLPEERMAP/QMGRMAP; CRD fields for MQSC keywords are deferred
	// (see docs/PHASE5_AUTH_SKETCH.md). Goldens assert ruleType-only SET CHLAUTH shape.
	cases := []struct {
		name     string
		ruleType mqadmin.ChannelAuthRuleType
		wantType string
	}{
		{"USERMAP", mqadmin.ChannelAuthRuleTypeUserMap, "USERMAP"},
		{"SSLPEERMAP", mqadmin.ChannelAuthRuleTypeSSLPeerMap, "SSLPEERMAP"},
		{"QMGRMAP", mqadmin.ChannelAuthRuleTypeQMGRMap, "QMGRMAP"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := buildSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{
				ChannelName: "APP.CH",
				RuleType:    tc.ruleType,
			}, "REPLACE")
			if err != nil {
				t.Fatalf("buildSetChannelAuthMQSC: %v", err)
			}
			want := "SET CHLAUTH('APP.CH') TYPE(" + tc.wantType + ") ACTION(REPLACE)"
			if cmd != want {
				t.Fatalf("got %q, want %q", cmd, want)
			}
		})
	}
}

func TestBuildSetChannelAuthMQSCRemove(t *testing.T) {
	cmd, err := buildSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP.SVRCONN.0TLS",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "REQUIRED",
		Description: "ignored on remove",
	}, "REMOVE")
	if err != nil {
		t.Fatalf("buildSetChannelAuthMQSC: %v", err)
	}
	want := "SET CHLAUTH('DEV.APP.SVRCONN.0TLS') TYPE(ADDRESSMAP) ADDRESS('*') ACTION(REMOVE)"
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
	want := "SET AUTHREC PROFILE('APP.ORDERS') OBJTYPE(QUEUE) PRINCIPAL('app') AUTHADD(GET,PUT)"
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
	want := "SET AUTHREC PROFILE('APP.ORDERS') OBJTYPE(QUEUE) GROUP('apps') AUTHRMV(ALL)"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestBuildSetAuthorityMQSCObjectTypes(t *testing.T) {
	cases := []struct {
		name       string
		objectType mqadmin.AuthorityObjectType
		wantObj    string
	}{
		{"CHANNEL", mqadmin.AuthorityObjectTypeChannel, "CHANNEL"},
		{"TOPIC", mqadmin.AuthorityObjectTypeTopic, "TOPIC"},
		{"QMGR", mqadmin.AuthorityObjectTypeQMGR, "QMGR"},
		{"NAMESPAC", mqadmin.AuthorityObjectTypeNamespace, "NAMESPAC"},
		{"PROCESS", mqadmin.AuthorityObjectTypeProcess, "PROCESS"},
		{"NLIST", mqadmin.AuthorityObjectTypeNList, "NLIST"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := buildSetAuthorityMQSC(mqadmin.AuthoritySpec{
				Profile:     "APP.PROFILE",
				ObjectType:  tc.objectType,
				Principal:   "app",
				Authorities: []string{"CONNECT"},
			}, false)
			if err != nil {
				t.Fatalf("buildSetAuthorityMQSC: %v", err)
			}
			want := "SET AUTHREC PROFILE('APP.PROFILE') OBJTYPE(" + tc.wantObj +
				") PRINCIPAL('app') AUTHADD(CONNECT)"
			if cmd != want {
				t.Fatalf("got %q, want %q", cmd, want)
			}
		})
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

func TestBuildDisplayChannelAuthMQSCWithAddress(t *testing.T) {
	cmd, err := buildDisplayChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "DISPLAY CHLAUTH('DEV.APP') TYPE(ADDRESSMAP) ADDRESS('*')"
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

func TestBuildDisplayAuthorityMQSCTopicObject(t *testing.T) {
	cmd, err := buildDisplayAuthorityMQSC(mqadmin.AuthoritySpec{
		Profile:    "SYSTEM.ADMIN.TOPIC",
		ObjectType: mqadmin.AuthorityObjectTypeTopic,
		Principal:  "app",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "OBJTYPE(TOPIC)") {
		t.Fatalf("cmd = %q", cmd)
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

func TestBuildDisplayChannelAuthMQSCBlockAddr(t *testing.T) {
	cmd, err := buildDisplayChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "*",
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockAddr,
		Address:     "192.0.2.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "DISPLAY CHLAUTH('*') TYPE(BLOCKADDR) ADDRESS('192.0.2.1')"
	if cmd != want {
		t.Fatalf("got %q, want %q", cmd, want)
	}
}

func TestChannelAuthStateFromAttributesBlockAddr(t *testing.T) {
	spec := mqadmin.ChannelAuthSpec{
		ChannelName: "*",
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockAddr,
	}
	state := channelAuthStateFromAttributes(spec, map[string]string{
		"address": "192.0.2.1", "descr": "block",
	})
	if state.Address != "192.0.2.1" || state.Description != "block" {
		t.Fatalf("state = %+v", state)
	}
}

func TestChannelAuthStateFromAttributesBlockUser(t *testing.T) {
	spec := mqadmin.ChannelAuthSpec{
		ChannelName: "CH1",
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockUser,
	}
	state := channelAuthStateFromAttributes(spec, map[string]string{
		"userlist": "nobody", "descr": "block",
	})
	if state.UserList != "nobody" || state.Description != "block" {
		t.Fatalf("state = %+v", state)
	}
}
