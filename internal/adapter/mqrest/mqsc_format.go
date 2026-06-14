package mqrest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

// FormatDefineQueueMQSC renders the DEFINE QLOCAL|QALIAS|QREMOTE ... REPLACE line
// equivalent to the DEFINE runCommandJSON the mqrest adapter sends. Intended as a
// debug/GitOps aid only; not authoritative for apply.
func FormatDefineQueueMQSC(spec mqadmin.QueueSpec) (string, error) {
	if err := validateQueueType(spec.Type); err != nil {
		return "", err
	}

	params := defineQueueParameters(spec)
	keyword := defineQueueMQSCKeyword(spec.Type)

	parts := []string{
		fmt.Sprintf("DEFINE %s('%s')", keyword, mqscQuote(spec.Name)),
		"REPLACE",
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		if k == attrReplace {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		parts = append(parts, formatMQSCAttribute(key, params[key]))
	}

	return strings.Join(parts, " "), nil
}

// FormatDefineTopicMQSC renders the DEFINE TOPIC ... REPLACE line equivalent to the
// DEFINE runCommandJSON the mqrest adapter sends.
func FormatDefineTopicMQSC(spec mqadmin.TopicSpec) (string, error) {
	params := defineTopicParameters(spec)
	return formatDefineObjectMQSC("TOPIC", spec.Name, params), nil
}

// FormatDefineChannelMQSC renders the DEFINE CHANNEL ... REPLACE line equivalent to the
// DEFINE runCommandJSON the mqrest adapter sends.
func FormatDefineChannelMQSC(spec mqadmin.ChannelSpec) (string, error) {
	params := defineChannelParameters(spec)
	return formatDefineObjectMQSC("CHANNEL", spec.Name, params), nil
}

// FormatSetChannelAuthMQSC renders the SET CHLAUTH ... ACTION(REPLACE) line the
// mqrest adapter applies.
func FormatSetChannelAuthMQSC(spec mqadmin.ChannelAuthSpec) (string, error) {
	return buildSetChannelAuthMQSC(spec, "REPLACE")
}

// FormatSetAuthorityMQSC renders the SET AUTHREC ... AUTHADD(...) line the mqrest
// adapter applies.
func FormatSetAuthorityMQSC(spec mqadmin.AuthoritySpec) (string, error) {
	return buildSetAuthorityMQSC(spec, false)
}

func formatDefineObjectMQSC(objectType, name string, params map[string]any) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == attrReplace {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, 2+len(keys))
	parts = append(parts,
		fmt.Sprintf("DEFINE %s('%s')", objectType, mqscQuote(name)),
		"REPLACE",
	)

	for _, key := range keys {
		parts = append(parts, formatMQSCAttribute(key, params[key]))
	}

	return strings.Join(parts, " ")
}

func defineQueueMQSCKeyword(qType mqadmin.QueueType) string {
	switch mqadmin.NormalizeQueueType(qType) {
	case mqadmin.QueueTypeAlias:
		return "QALIAS"
	case mqadmin.QueueTypeRemote:
		return "QREMOTE"
	default:
		return "QLOCAL"
	}
}

func formatMQSCAttribute(key string, value any) string {
	mqscKey := strings.ToUpper(key)
	switch v := value.(type) {
	case int:
		return fmt.Sprintf("%s(%d)", mqscKey, v)
	case int64:
		return fmt.Sprintf("%s(%d)", mqscKey, v)
	case float64:
		return fmt.Sprintf("%s(%g)", mqscKey, v)
	default:
		return fmt.Sprintf("%s('%s')", mqscKey, mqscQuote(fmt.Sprint(v)))
	}
}
