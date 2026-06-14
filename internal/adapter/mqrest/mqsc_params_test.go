package mqrest

import (
	"strings"
	"testing"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
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
	want := map[string]struct{}{"maxinst": {}, "maxinstc": {}, "sslciph": {}, "sslcauth": {}}
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

func TestQueueLocalDisplayParametersOmitExtendedAttrsOn94(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"share", "defopts", "bothresh", "boqname", "usage"} {
		for _, q := range queueLocalDisplayParameters {
			if q == p {
				t.Fatalf("%q must not be in queueLocalDisplayParameters on mqweb 9.4 (MQWB0120E)", p)
			}
		}
	}
}

func TestDriftCheckKeyExports(t *testing.T) {
	t.Parallel()
	if len(QueueDriftCheckKeys(mqadmin.QueueTypeLocal)) == 0 {
		t.Fatal("expected local queue drift keys")
	}
	if len(TopicDriftCheckKeys()) == 0 {
		t.Fatal("expected topic drift keys")
	}
	if len(ChannelDriftCheckKeys()) == 0 {
		t.Fatal("expected channel drift keys")
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

func TestMapTopicRESTParameters_PubSubUppercase(t *testing.T) {
	t.Parallel()
	params := map[string]any{"pub": "enabled", "sub": "disabled"}
	mapTopicRESTParameters(params)
	if params["pub"] != "ENABLED" || params["sub"] != "DISABLED" {
		t.Fatalf("params = %v", params)
	}
}

func TestNormalizeTopicAttributes(t *testing.T) {
	t.Parallel()
	attrs := map[string]string{strings.ToLower(attrTopicStr): "retail/orders"}
	normalizeTopicAttributes(attrs)
	if attrs[attrTopstr] != "retail/orders" {
		t.Fatalf("attrs = %v", attrs)
	}
}

func TestNormalizeQueueAttributes(t *testing.T) {
	t.Parallel()
	t.Run("alias maps target to targq", func(t *testing.T) {
		t.Parallel()
		attrs := map[string]string{"target": "APP.BASE"}
		normalizeQueueAttributes(attrs, mqadmin.QueueTypeAlias)
		if attrs["targq"] != "APP.BASE" {
			t.Fatalf("attrs = %v", attrs)
		}
	})
	t.Run("remote maps mqweb names", func(t *testing.T) {
		t.Parallel()
		attrs := map[string]string{
			"remotequeue":       "REMOTE.Q",
			"remotemanager":     "QM2",
			"transmissionqueue": "XMIT.Q",
		}
		normalizeQueueAttributes(attrs, mqadmin.QueueTypeRemote)
		if attrs["rname"] != "REMOTE.Q" || attrs["rqmname"] != "QM2" || attrs["xmitq"] != "XMIT.Q" {
			t.Fatalf("attrs = %v", attrs)
		}
	})
	t.Run("local is no-op", func(t *testing.T) {
		t.Parallel()
		attrs := map[string]string{"maxdepth": "5000"}
		normalizeQueueAttributes(attrs, mqadmin.QueueTypeLocal)
		if attrs["maxdepth"] != "5000" {
			t.Fatalf("attrs = %v", attrs)
		}
	})
}

func TestQueueDisplayParametersByType(t *testing.T) {
	t.Parallel()
	if got := queueDisplayParameters(mqadmin.QueueTypeAlias); len(got) == 0 || got[0] != "targq" {
		t.Fatalf("alias display = %v", got)
	}
	if got := queueDisplayParameters(mqadmin.QueueTypeRemote); len(got) == 0 || got[0] != "rname" {
		t.Fatalf("remote display = %v", got)
	}
}

func TestDefineObjectParameters_InvalidNumericStaysString(t *testing.T) {
	t.Parallel()
	params := defineObjectParameters(map[string]string{attrMaxDepth: "not-a-number"}, queueNumericParameters)
	if params[attrMaxDepth] != "not-a-number" {
		t.Fatalf("params = %v", params)
	}
}

func TestQueueDisplayRequestUsesQualifier(t *testing.T) {
	t.Parallel()
	req := queueDisplayRequest(mqadmin.QueueSpec{Name: "APP.ALIAS", Type: mqadmin.QueueTypeAlias})
	if req.Qualifier != "qalias" || req.Name != "APP.ALIAS" {
		t.Fatalf("request = %+v", req)
	}
}

func TestChannelDisplayRequestIncludesChltype(t *testing.T) {
	t.Parallel()
	req := channelDisplayRequest("ORDERS.APP", mqadmin.ChannelTypeSvrconn)
	if req.Qualifier != "channel" || req.Name != "ORDERS.APP" {
		t.Fatalf("request = %+v", req)
	}
	if req.Parameters["chltype"] != "svrconn" {
		t.Fatalf("chltype = %v", req.Parameters["chltype"])
	}
	if len(req.ResponseParameters) == 0 {
		t.Fatal("expected response parameters")
	}
}
