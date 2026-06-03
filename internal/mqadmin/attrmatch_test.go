package mqadmin

import "testing"

func TestAttributeValueMatches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		key, desired, observed string
		want                   bool
	}{
		{"pub", "enabled", "ENABLED", true},
		{"maxdepth", "5000", "5000", true},
		{"maxdepth", "5000", "5000 ", true},
		{"sharecnv", "10", "10", true},
		{"descr", "a", "b", false},
		{"topstr", "retail/orders", "retail/orders", true},
	}
	for _, tc := range cases {
		if got := AttributeValueMatches(tc.key, tc.desired, tc.observed); got != tc.want {
			t.Errorf("%s %q vs %q: got %v want %v", tc.key, tc.desired, tc.observed, got, tc.want)
		}
	}
}

func TestAttributesNeedUpdate(t *testing.T) {
	t.Parallel()
	checkKeys := []string{"maxdepth", "pub"}
	desired := map[string]string{"maxdepth": "5000", "pub": "enabled"}
	observed := map[string]string{"maxdepth": "5000", "pub": "ENABLED"}
	if AttributesNeedUpdate(desired, observed, checkKeys) {
		t.Fatal("expected no update")
	}
	observed["maxdepth"] = "1000"
	if !AttributesNeedUpdate(desired, observed, checkKeys) {
		t.Fatal("expected update on drift")
	}
}

func TestAttributesNeedUpdate_SkipsNonDriftKeys(t *testing.T) {
	t.Parallel()
	checkKeys := []string{"maxdepth"}
	desired := map[string]string{"maxdepth": "5000", "maxmsglen": "4194304"}
	observed := map[string]string{"maxdepth": "5000"}
	if AttributesNeedUpdate(desired, observed, checkKeys) {
		t.Fatal("define-only keys should not trigger drift")
	}
}

func TestAttributeDriftsForKeys(t *testing.T) {
	t.Parallel()
	drifts := AttributeDriftsForKeys(
		map[string]string{"maxdepth": "5000", "descr": "a"},
		map[string]string{"maxdepth": "1000", "descr": "a"},
		[]string{"maxdepth", "descr"},
	)
	if len(drifts) != 1 || drifts[0].Key != "maxdepth" {
		t.Fatalf("drifts = %+v", drifts)
	}
}

func TestFormatAttributeDriftMessage(t *testing.T) {
	t.Parallel()
	msg := FormatAttributeDriftMessage([]AttributeDrift{
		{Key: "maxdepth", Desired: "5000", Observed: "1000"},
	})
	if msg != `attribute drift: maxdepth: desired "5000" observed "1000"` {
		t.Fatalf("msg = %q", msg)
	}
}

func TestNormalizeAttrKey(t *testing.T) {
	t.Parallel()
	if got := NormalizeAttrKey("TopicStr"); got != "topstr" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeAttrKey("MaxDepth"); got != "maxdepth" {
		t.Fatalf("got %q", got)
	}
}

func TestAttributeValueMatches_NumericNormalization(t *testing.T) {
	t.Parallel()
	if !AttributeValueMatches("maxdepth", "05000", "5000") {
		t.Fatal("expected numeric equivalence")
	}
	if AttributeValueMatches("maxdepth", "bad", "5000") {
		t.Fatal("expected mismatch for non-numeric desired")
	}
}

func TestFormatAttributeDriftMessage_Empty(t *testing.T) {
	t.Parallel()
	if FormatAttributeDriftMessage(nil) != "" {
		t.Fatal("expected empty message")
	}
}

func TestAttributeValueMatches_ExtendedKeys(t *testing.T) {
	t.Parallel()
	if !AttributeValueMatches("share", "yes", "YES") {
		t.Fatal("share should be case-insensitive")
	}
	if !AttributeValueMatches("sslcauth", "required", "REQUIRED") {
		t.Fatal("sslcauth should be case-insensitive")
	}
	if !AttributeValueMatches("bothresh", "5", "05") {
		t.Fatal("bothresh should normalize numerically")
	}
}
