package mqadmin

import (
	"strconv"
	"strings"
)

const (
	attrKeyPub       = "pub"
	attrKeySub       = "sub"
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
	attrKeyPub:      {},
	attrKeySub:      {},
	attrKeyGet:      {},
	attrKeyPut:      {},
	attrKeyDefpsist: {},
	attrKeyTrptype:  {},
}

var numericAttrKeys = map[string]struct{}{
	attrKeyMaxdepth:  {},
	attrKeyMaxmsglen: {},
	attrKeyMaxmsgl:   {},
	attrKeySharecnv:  {},
	attrKeyMaxinst:   {},
	attrKeyMaxinstc:  {},
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

// AttributesNeedUpdate returns true when any desired attribute differs from observed.
func AttributesNeedUpdate(desired map[string]string, observed map[string]string) bool {
	for k, v := range desired {
		key := NormalizeAttrKey(k)
		if !AttributeValueMatches(key, v, observed[key]) {
			return true
		}
	}
	return false
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
