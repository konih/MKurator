package mqrest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/konradheimel/kurator/internal/mqadmin"
)

const attrMaxDepth = "maxdepth"
const attrDescr = "descr"
const attrMaxMsgl = "maxmsgl"
const attrTopstr = "topstr"
const attrTopicStr = "topicStr" // mqweb runCommandJSON name for TOPSTR

// queueDisplayParameters lists attributes safe for runCommandJSON DISPLAY qlocal
// on IBM MQ 9.4.x. Some keywords (e.g. maxmsglen) are rejected by mqweb with
// MQWB0120E even though they are valid on DEFINE.
var queueLocalDisplayParameters = []string{
	attrMaxDepth, attrDescr, "defpsist", "get", "put",
}

var queueAliasDisplayParameters = []string{
	"targq", "targtype", attrDescr,
}

var queueRemoteDisplayParameters = []string{
	"rname", "rqmname", "xmitq", attrDescr,
}

// queueNumericParameters are coerced to JSON numbers for runCommandJSON DEFINE.
var queueNumericParameters = map[string]struct{}{
	attrMaxDepth: {},
	"maxmsglen":  {},
}

const (
	attrSharecnv = "sharecnv"
	attrMaxInst  = "maxinst"
	attrMaxInstc = "maxinstc"
)

var channelNumericParameters = map[string]struct{}{
	attrSharecnv: {},
	attrMaxMsgl:  {},
	attrMaxInst:  {},
	attrMaxInstc: {},
}

// topicDisplayParameters lists attributes safe for DISPLAY topic on IBM MQ 9.4.x
// mqweb. pubscope/subscope are included for drift on 9.4; omit from this slice if
// your QM returns MQWB0120E (see docs/ATTRIBUTE_RECONCILIATION.md).
var topicDisplayParameters = []string{
	attrTopicStr, attrDescr, "pub", "sub", "defpsist", "pubscope", "subscope",
}

var channelDisplayParameters = []string{
	attrDescr, "trptype", attrSharecnv, attrMaxMsgl, "mcauser", attrMaxInst, attrMaxInstc,
}

func defineTopicParameters(spec mqadmin.TopicSpec) map[string]any {
	params := defineObjectParameters(spec.Attributes, nil)
	mapTopicRESTParameters(params)
	return params
}

// mapTopicRESTParameters translates CRD/MQSC attribute names to mqweb JSON names.
func mapTopicRESTParameters(params map[string]any) {
	if v, ok := params[attrTopstr]; ok {
		params[attrTopicStr] = v
		delete(params, attrTopstr)
	}
	for _, key := range []string{"pub", "sub"} {
		if v, ok := params[key]; ok {
			params[key] = strings.ToUpper(fmt.Sprint(v))
		}
	}
}

func normalizeTopicAttributes(attrs map[string]string) {
	if v, ok := attrs[strings.ToLower(attrTopicStr)]; ok {
		attrs[attrTopstr] = v
	}
}

func defineChannelParameters(spec mqadmin.ChannelSpec) map[string]any {
	params := defineObjectParameters(spec.Attributes, channelNumericParameters)
	if spec.Type != "" {
		params["chltype"] = string(spec.Type)
	}
	return params
}

func defineObjectParameters(
	attrs map[string]string,
	numericKeys map[string]struct{},
) map[string]any {
	params := map[string]any{"replace": mqscReplaceYes}
	for k, v := range attrs {
		key := strings.ToLower(k)
		if numericKeys != nil {
			if _, numeric := numericKeys[key]; numeric {
				if n, err := strconv.Atoi(v); err == nil {
					params[key] = n
					continue
				}
			}
		}
		params[key] = v
	}
	return params
}

func defineQueueParameters(spec mqadmin.QueueSpec) map[string]any {
	return defineObjectParameters(spec.Attributes, queueNumericParameters)
}

func queueQualifier(qType mqadmin.QueueType) string {
	switch mqadmin.NormalizeQueueType(qType) {
	case mqadmin.QueueTypeAlias:
		return qualifierQAlias
	case mqadmin.QueueTypeRemote:
		return qualifierQRemote
	default:
		return qualifierQLocal
	}
}

func queueDisplayParameters(qType mqadmin.QueueType) []string {
	switch mqadmin.NormalizeQueueType(qType) {
	case mqadmin.QueueTypeAlias:
		return append([]string(nil), queueAliasDisplayParameters...)
	case mqadmin.QueueTypeRemote:
		return append([]string(nil), queueRemoteDisplayParameters...)
	default:
		return append([]string(nil), queueLocalDisplayParameters...)
	}
}

func queueDisplayRequest(spec mqadmin.QueueSpec) runCommandJSONRequest {
	return runCommandJSONRequest{
		Type:               mqscType,
		Command:            mqscCommandDisplay,
		Qualifier:          queueQualifier(spec.Type),
		Name:               spec.Name,
		ResponseParameters: queueDisplayParameters(spec.Type),
	}
}

func channelDisplayRequest(name string, chlType mqadmin.ChannelType) runCommandJSONRequest {
	params := map[string]any{}
	if chlType != "" {
		params["chltype"] = string(chlType)
	}
	return runCommandJSONRequest{
		Type:               mqscType,
		Command:            mqscCommandDisplay,
		Qualifier:          qualifierChannel,
		Name:               name,
		Parameters:         params,
		ResponseParameters: append([]string(nil), channelDisplayParameters...),
	}
}
