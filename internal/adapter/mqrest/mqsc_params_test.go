package mqrest

import (
	"testing"

	"github.com/konradheimel/kurator/internal/mqadmin"
)

func TestDefineQueueParameters(t *testing.T) {
	t.Parallel()
	params := defineQueueParameters(mqadmin.QueueSpec{
		Name: "APP.ORDERS",
		Attributes: map[string]string{
			attrMaxDepth: "5000",
			"descr":      "orders",
		},
	})
	if params["replace"] != "yes" {
		t.Fatalf("replace = %v", params["replace"])
	}
	if params[attrMaxDepth] != 5000 {
		t.Fatalf("maxdepth should be int 5000, got %T(%v)", params[attrMaxDepth], params[attrMaxDepth])
	}
	if params["descr"] != "orders" {
		t.Fatalf("descr = %v", params["descr"])
	}
}

func TestQueueDisplayParametersExcludeMaxmsglen(t *testing.T) {
	t.Parallel()
	for _, p := range queueLocalDisplayParameters {
		if p == "maxmsglen" {
			t.Fatal("maxmsglen must not be in display parameters for mqweb 9.4")
		}
	}
}

func TestQueueQualifier(t *testing.T) {
	t.Parallel()
	if queueQualifier(mqadmin.QueueTypeLocal) != "qlocal" {
		t.Fatal("local")
	}
	if queueQualifier(mqadmin.QueueTypeAlias) != "qalias" {
		t.Fatal("alias")
	}
	if queueQualifier(mqadmin.QueueTypeRemote) != "qremote" {
		t.Fatal("remote")
	}
}

func TestDefineChannelParameters(t *testing.T) {
	t.Parallel()
	params := defineChannelParameters(mqadmin.ChannelSpec{
		Name: "ORDERS.APP",
		Type: mqadmin.ChannelTypeSvrconn,
		Attributes: map[string]string{
			"maxmsgl": "4194304",
			"trptype": "tcp",
		},
	})
	if params["chltype"] != "svrconn" {
		t.Fatalf("chltype = %v", params["chltype"])
	}
	if params["maxmsgl"] != 4194304 {
		t.Fatalf("maxmsgl should be int, got %T(%v)", params["maxmsgl"], params["maxmsgl"])
	}
}

func TestChannelDisplayParametersIncludeConnectionLimits(t *testing.T) {
	t.Parallel()
	want := map[string]struct{}{"maxinst": {}, "maxinstc": {}}
	for k := range want {
		found := false
		for _, p := range channelDisplayParameters {
			if p == k {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%q missing from channelDisplayParameters", k)
		}
	}
}

func TestTopicDisplayParametersIncludeScope(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"pubscope", "subscope"} {
		found := false
		for _, q := range topicDisplayParameters {
			if q == p {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%q missing from topicDisplayParameters", p)
		}
	}
}

func TestDefineTopicParameters(t *testing.T) {
	t.Parallel()
	params := defineTopicParameters(mqadmin.TopicSpec{
		Name: "RETAIL.ORDERS",
		Attributes: map[string]string{
			"topstr": "retail/orders",
		},
	})
	if params["replace"] != "yes" || params[attrTopicStr] != "retail/orders" {
		t.Fatalf("params = %v", params)
	}
	if _, ok := params[attrTopstr]; ok {
		t.Fatal("topstr should be mapped to topicStr for mqweb")
	}
}
