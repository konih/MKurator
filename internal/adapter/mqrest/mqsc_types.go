package mqrest

import (
	"fmt"
	"strings"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

// runCommandRequest matches docs/schemas/mqsc-runcommand.schema.json runCommand.
type runCommandRequest struct {
	Type       string `json:"type"`
	Parameters struct {
		Command string `json:"command"`
	} `json:"parameters"`
}

// runCommandJSONRequest matches docs/schemas/mqsc-runcommand.schema.json runCommandJSON.
type runCommandJSONRequest struct {
	Type               string         `json:"type"`
	Command            string         `json:"command"`
	Qualifier          string         `json:"qualifier,omitempty"`
	Name               string         `json:"name,omitempty"`
	ResponseParameters []string       `json:"responseParameters,omitempty"`
	Parameters         map[string]any `json:"parameters,omitempty"`
}

type commandResponseItem struct {
	CompletionCode int            `json:"completionCode"`
	ReasonCode     int            `json:"reasonCode"`
	Message        []string       `json:"message"`
	Text           []string       `json:"text"`
	Parameters     map[string]any `json:"parameters"`
}

type mqscResponse struct {
	CommandResponse       []commandResponseItem `json:"commandResponse"`
	OverallCompletionCode int                   `json:"overallCompletionCode"`
	OverallReasonCode     int                   `json:"overallReasonCode"`
	OverAllReasonCode     int                   `json:"overAllReasonCode"` // typo variant in schema
	Error                 []restErrorItem       `json:"error"`
}

type restErrorItem struct {
	Message        string `json:"message"`
	Explanation    string `json:"explanation"`
	ReasonCode     int    `json:"reasonCode"`
	CompletionCode int    `json:"completionCode"`
}

func (r *mqscResponse) overallFailed() bool {
	if r.OverallCompletionCode != 0 {
		return true
	}
	for _, cr := range r.CommandResponse {
		if cr.CompletionCode != 0 {
			return true
		}
	}
	return len(r.Error) > 0
}

func (r *mqscResponse) isObjectMissing() bool {
	for _, cr := range r.CommandResponse {
		for _, msg := range append(cr.Message, cr.Text...) {
			if strings.Contains(strings.ToUpper(msg), "AMQ8147") ||
				strings.Contains(strings.ToUpper(msg), "AMQ8101") ||
				strings.Contains(strings.ToUpper(msg), "AMQ8884") ||
				strings.Contains(strings.ToUpper(msg), "AMQ8958") ||
				strings.Contains(strings.ToLower(msg), "not found") {
				return true
			}
		}
	}
	return false
}

func (r *mqscResponse) terminalError(msg string) error {
	detail := r.firstMessage()
	return &mqadmin.TerminalError{
		Reason:  "MQSCError",
		Message: fmt.Sprintf("%s: %s", msg, detail),
	}
}

func (r *mqscResponse) firstObjectAttributes() (map[string]string, error) {
	if len(r.CommandResponse) == 0 {
		return nil, &mqadmin.NotFoundError{Object: ""}
	}
	attrs := map[string]string{}
	for _, cr := range r.CommandResponse {
		for k, v := range cr.Parameters {
			attrs[strings.ToLower(k)] = fmt.Sprint(v)
		}
	}
	if len(attrs) == 0 && r.overallFailed() {
		if r.isObjectMissing() {
			return nil, &mqadmin.NotFoundError{Object: ""}
		}
		return nil, r.terminalError("display object")
	}
	return attrs, nil
}

func (r *mqscResponse) firstMessage() string {
	for _, cr := range r.CommandResponse {
		if len(cr.Message) > 0 {
			return cr.Message[0]
		}
		if len(cr.Text) > 0 {
			return cr.Text[0]
		}
	}
	if len(r.Error) > 0 {
		return r.Error[0].Message
	}
	return "unknown mqsc error"
}
