package mqadmin

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	AttrKeyPub       = "pub"
	AttrKeySub       = "sub"
	attrKeyGet       = "get"
	attrKeyPut       = "put"
	attrKeyDefpsist  = "defpsist"
	attrKeyTrptype   = "trptype"
	attrKeyMaxdepth  = "maxdepth"
	attrKeyMaxmsglen = "maxmsglen"
	attrKeyMaxmsgl   = "maxmsgl"
	attrKeySharecnv  = "sharecnv"
	attrKeyMaxinst   = "maxinst"
	attrKeyMaxinstc  = "maxinstc"
	attrKeyTopstr    = "topstr"
	attrKeyTopicstr  = "topicstr"
)

var caseInsensitiveAttrKeys = map[string]struct{}{
	AttrKeyPub:      {},
	AttrKeySub:      {},
	attrKeyGet:      {},
	attrKeyPut:      {},
	attrKeyDefpsist: {},
	attrKeyTrptype:  {},
	"share":         {},
	"defopts":       {},
	"usage":         {},
	"sslcauth":      {},
}

var numericAttrKeys = map[string]struct{}{
	attrKeyMaxdepth:  {},
	attrKeyMaxmsglen: {},
	attrKeyMaxmsgl:   {},
	attrKeySharecnv:  {},
	attrKeyMaxinst:   {},
	attrKeyMaxinstc:  {},
	"bothresh":       {},
}

// AttributeDrift describes one drift-checked attribute that differs from spec.
type AttributeDrift struct {
	Key      string
	Desired  string
	Observed string
}

// NormalizeAttrKey lowercases MQSC attribute keys and applies mqweb aliases.
func NormalizeAttrKey(key string) string {
	key = strings.ToLower(key)
	if key == attrKeyTopicstr {
		return attrKeyTopstr
	}
	return key
}

// AttributeValueMatches reports whether desired and observed MQ attribute values
// are equivalent for drift detection.
func AttributeValueMatches(key, desired, observed string) bool {
	key = NormalizeAttrKey(key)
	if _, ok := caseInsensitiveAttrKeys[key]; ok {
		return strings.EqualFold(strings.TrimSpace(desired), strings.TrimSpace(observed))
	}
	if _, ok := numericAttrKeys[key]; ok {
		return normalizeNumericString(desired) == normalizeNumericString(observed)
	}
	return strings.TrimSpace(desired) == strings.TrimSpace(observed)
}

// AttributesNeedUpdate returns true when any desired drift-checked attribute differs from observed.
func AttributesNeedUpdate(desired map[string]string, observed map[string]string, checkKeys []string) bool {
	return len(AttributeDriftsForKeys(desired, observed, checkKeys)) > 0
}

// AttributeDriftsForKeys returns drift details for desired keys in the check list.
func AttributeDriftsForKeys(desired, observed map[string]string, checkKeys []string) []AttributeDrift {
	allowed := driftCheckKeySet(checkKeys)
	var drifts []AttributeDrift
	for k, v := range desired {
		key := NormalizeAttrKey(k)
		if _, ok := allowed[key]; !ok {
			continue
		}
		obs := observed[key]
		if AttributeValueMatches(key, v, obs) {
			continue
		}
		drifts = append(drifts, AttributeDrift{Key: key, Desired: v, Observed: obs})
	}
	return drifts
}

// FormatAttributeDriftMessage summarizes attribute drift for status conditions.
func FormatAttributeDriftMessage(drifts []AttributeDrift) string {
	if len(drifts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(drifts))
	for _, d := range drifts {
		parts = append(parts, fmt.Sprintf("%s: desired %q observed %q", d.Key, d.Desired, d.Observed))
	}
	return "attribute drift: " + strings.Join(parts, "; ")
}

func driftCheckKeySet(checkKeys []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(checkKeys))
	for _, k := range checkKeys {
		allowed[NormalizeAttrKey(k)] = struct{}{}
	}
	return allowed
}

func normalizeNumericString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	return strconv.Itoa(n)
}
