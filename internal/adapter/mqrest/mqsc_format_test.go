package mqrest

import (
	"strings"
	"testing"

	"github.com/konih/kurator/internal/mqadmin"
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
