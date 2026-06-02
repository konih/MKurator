package mqrest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/konih/kurator/internal/mqadmin"
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
		if k == "replace" {
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
