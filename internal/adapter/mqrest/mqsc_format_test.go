package mqrest

import (
	"strings"
	"testing"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

func TestFormatDefineQueueMQSC_Local(t *testing.T) {
	t.Parallel()
	got, err := FormatDefineQueueMQSC(mqadmin.QueueSpec{
		Name: "APP.ORDERS",
		Type: mqadmin.QueueTypeLocal,
		Attributes: map[string]string{
			attrMaxDepth: "5000",
			"descr":      "orders",
		},
	})
	if err != nil {
		t.Fatalf("FormatDefineQueueMQSC: %v", err)
	}
	want := "DEFINE QLOCAL('APP.ORDERS') REPLACE DESCR('orders') MAXDEPTH(5000)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatDefineQueueMQSC_Alias(t *testing.T) {
	t.Parallel()
	got, err := FormatDefineQueueMQSC(mqadmin.QueueSpec{
		Name: "APP.ALIAS",
		Type: mqadmin.QueueTypeAlias,
		Attributes: map[string]string{
			attrTargq: "APP.BASE",
		},
	})
	if err != nil {
		t.Fatalf("FormatDefineQueueMQSC: %v", err)
	}
	if !strings.HasPrefix(got, "DEFINE QALIAS('APP.ALIAS') REPLACE ") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "TARGQ('APP.BASE')") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatDefineQueueMQSC_UnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := FormatDefineQueueMQSC(mqadmin.QueueSpec{
		Name: "APP.ORDERS",
		Type: mqadmin.QueueType("model"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatDefineQueueMQSC_QuotesObjectName(t *testing.T) {
	t.Parallel()
	got, err := FormatDefineQueueMQSC(mqadmin.QueueSpec{
		Name: "IT'S.Q",
		Type: mqadmin.QueueTypeLocal,
	})
	if err != nil {
		t.Fatalf("FormatDefineQueueMQSC: %v", err)
	}
	if !strings.Contains(got, "QLOCAL('IT''S.Q')") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatDefineTopicMQSC(t *testing.T) {
	t.Parallel()
	got, err := FormatDefineTopicMQSC(mqadmin.TopicSpec{
		Name: "RETAIL.ORDERS",
		Attributes: map[string]string{
			attrTopstr: "retail/orders",
			"descr":    "orders topic",
		},
	})
	if err != nil {
		t.Fatalf("FormatDefineTopicMQSC: %v", err)
	}
	want := "DEFINE TOPIC('RETAIL.ORDERS') REPLACE DESCR('orders topic') TOPICSTR('retail/orders')"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatDefineChannelMQSC(t *testing.T) {
	t.Parallel()
	got, err := FormatDefineChannelMQSC(mqadmin.ChannelSpec{
		Name: "ORDERS.APP",
		Type: mqadmin.ChannelTypeSvrconn,
		Attributes: map[string]string{
			attrTrptype: "tcp",
			"descr":     "app channel",
		},
	})
	if err != nil {
		t.Fatalf("FormatDefineChannelMQSC: %v", err)
	}
	want := "DEFINE CHANNEL('ORDERS.APP') REPLACE CHLTYPE('svrconn') DESCR('app channel') TRPTYPE('tcp')"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatSetChannelAuthMQSC(t *testing.T) {
	t.Parallel()
	got, err := FormatSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP.SVRCONN.0TLS",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "REQUIRED",
	})
	if err != nil {
		t.Fatalf("FormatSetChannelAuthMQSC: %v", err)
	}
	want := "SET CHLAUTH('DEV.APP.SVRCONN.0TLS') TYPE(ADDRESSMAP) ADDRESS('*') " +
		"USERSRC(CHANNEL) CHCKCLNT(REQUIRED) ACTION(REPLACE)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatSetChannelAuthMQSCBlockUser(t *testing.T) {
	t.Parallel()
	got, err := FormatSetChannelAuthMQSC(mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP.SVRCONN.0TLS",
		RuleType:    mqadmin.ChannelAuthRuleTypeBlockUser,
		UserList:    "nobody",
		Description: "Deny privileged user IDs",
	})
	if err != nil {
		t.Fatalf("FormatSetChannelAuthMQSC: %v", err)
	}
	want := "SET CHLAUTH('DEV.APP.SVRCONN.0TLS') TYPE(BLOCKUSER) USERLIST('nobody') " +
		"DESCR('Deny privileged user IDs') ACTION(REPLACE)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatDefineQueueMQSC_Remote(t *testing.T) {
	t.Parallel()
	got, err := FormatDefineQueueMQSC(mqadmin.QueueSpec{
		Name: "APP.REMOTE",
		Type: mqadmin.QueueTypeRemote,
		Attributes: map[string]string{
			"xmitq": "SYSTEM.DEF.XMIT.QUEUE",
		},
	})
	if err != nil {
		t.Fatalf("FormatDefineQueueMQSC: %v", err)
	}
	if !strings.HasPrefix(got, "DEFINE QREMOTE('APP.REMOTE') REPLACE ") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatMQSCAttributeNumeric(t *testing.T) {
	t.Parallel()
	if got := formatMQSCAttribute("maxdepth", 5000); got != "MAXDEPTH(5000)" {
		t.Fatalf("got %q", got)
	}
	if got := formatMQSCAttribute("maxdepth", int64(5000)); got != "MAXDEPTH(5000)" {
		t.Fatalf("got %q", got)
	}
	if got := formatMQSCAttribute("maxdepth", float64(5000)); got != "MAXDEPTH(5000)" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatSetAuthorityMQSC(t *testing.T) {
	t.Parallel()
	got, err := FormatSetAuthorityMQSC(mqadmin.AuthoritySpec{
		Profile:     "APP.ORDERS",
		ObjectType:  mqadmin.AuthorityObjectTypeQueue,
		Principal:   "app",
		Authorities: []string{"GET", "PUT"},
	})
	if err != nil {
		t.Fatalf("FormatSetAuthorityMQSC: %v", err)
	}
	want := "SET AUTHREC PROFILE('APP.ORDERS') OBJTYPE(QUEUE) PRINCIPAL('app') AUTHADD(GET,PUT)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
