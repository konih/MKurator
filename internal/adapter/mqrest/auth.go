package mqrest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/konih/kurator/internal/metrics"
	"github.com/konih/kurator/internal/mqadmin"
)

const invalidSpecReason = "InvalidSpec"

// SetChannelAuth applies SET CHLAUTH ... ACTION(REPLACE).
func (c *Client) SetChannelAuth(ctx context.Context, spec mqadmin.ChannelAuthSpec) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpSetChannelAuth, err) }()

	cmd, buildErr := buildSetChannelAuthMQSC(spec, "REPLACE")
	if buildErr != nil {
		err = buildErr
		return err
	}
	err = c.RunMQSC(ctx, cmd)
	return err
}

// DeleteChannelAuth applies SET CHLAUTH ... ACTION(REMOVE).
func (c *Client) DeleteChannelAuth(ctx context.Context, spec mqadmin.ChannelAuthSpec) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpDeleteChannelAuth, err) }()

	cmd, buildErr := buildSetChannelAuthMQSC(spec, "REMOVE")
	if buildErr != nil {
		err = buildErr
		return err
	}
	err = c.runMQSCAllowNotFound(ctx, cmd)
	return err
}

// SetAuthority applies SET AUTHREC ... AUTHADD(...) ACTION(REPLACE).
func (c *Client) SetAuthority(ctx context.Context, spec mqadmin.AuthoritySpec) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpSetAuthority, err) }()

	cmd, buildErr := buildSetAuthorityMQSC(spec, false)
	if buildErr != nil {
		err = buildErr
		return err
	}
	err = c.RunMQSC(ctx, cmd)
	return err
}

// DeleteAuthority applies SET AUTHREC ... AUTHRMV(ALL).
func (c *Client) DeleteAuthority(ctx context.Context, spec mqadmin.AuthoritySpec) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpDeleteAuthority, err) }()

	cmd, buildErr := buildSetAuthorityMQSC(spec, true)
	if buildErr != nil {
		err = buildErr
		return err
	}
	err = c.runMQSCAllowNotFound(ctx, cmd)
	return err
}

// GetChannelAuth returns observed CHLAUTH attributes via DISPLAY MQSC.
func (c *Client) GetChannelAuth(ctx context.Context, spec mqadmin.ChannelAuthSpec) (*mqadmin.ChannelAuthState, error) {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpGetChannelAuth, err) }()

	cmd, buildErr := buildDisplayChannelAuthMQSC(spec)
	if buildErr != nil {
		err = buildErr
		return nil, err
	}
	resp, err := c.runDisplayMQSC(ctx, cmd, spec.ChannelName)
	if err != nil {
		return nil, err
	}
	return channelAuthStateFromAttributes(spec, resp), nil
}

// GetAuthority returns observed OAM authorities via DISPLAY AUTHREC MQSC.
func (c *Client) GetAuthority(ctx context.Context, spec mqadmin.AuthoritySpec) (*mqadmin.AuthorityState, error) {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpGetAuthority, err) }()

	cmd, buildErr := buildDisplayAuthorityMQSC(spec)
	if buildErr != nil {
		err = buildErr
		return nil, err
	}
	resp, err := c.runDisplayMQSC(ctx, cmd, spec.Profile)
	if err != nil {
		return nil, err
	}
	return authorityStateFromAttributes(spec, resp), nil
}

func (c *Client) runDisplayMQSC(ctx context.Context, command, object string) (map[string]string, error) {
	body := runCommandRequest{Type: "runCommand"}
	body.Parameters.Command = command
	parsed, err := c.postMQSC(ctx, body)
	if err != nil {
		if nf := (*mqadmin.NotFoundError)(nil); errors.As(err, &nf) {
			return nil, &mqadmin.NotFoundError{Object: object}
		}
		return nil, err
	}
	attrs, err := parsed.firstObjectAttributes()
	if err != nil {
		if nf := (*mqadmin.NotFoundError)(nil); errors.As(err, &nf) {
			return nil, &mqadmin.NotFoundError{Object: object}
		}
		return nil, err
	}
	return attrs, nil
}

func channelAuthStateFromAttributes(
	spec mqadmin.ChannelAuthSpec,
	attrs map[string]string,
) *mqadmin.ChannelAuthState {
	return &mqadmin.ChannelAuthState{
		ChannelName: spec.ChannelName,
		RuleType:    spec.RuleType,
		Address:     attrs["address"],
		UserSource:  attrs["usersrc"],
		CheckClient: attrs["chckclnt"],
		Description: attrs["descr"],
	}
}

func authorityStateFromAttributes(
	spec mqadmin.AuthoritySpec,
	attrs map[string]string,
) *mqadmin.AuthorityState {
	state := &mqadmin.AuthorityState{
		Profile:    spec.Profile,
		ObjectType: spec.ObjectType,
		Principal:  spec.Principal,
		Group:      spec.Group,
	}
	if authlist := attrs["authlist"]; authlist != "" {
		for part := range strings.SplitSeq(authlist, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				state.Authorities = append(state.Authorities, part)
			}
		}
	}
	return state
}

func buildDisplayChannelAuthMQSC(spec mqadmin.ChannelAuthSpec) (string, error) {
	if spec.ChannelName == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "channel name is required"}
	}
	if spec.RuleType == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "rule type is required"}
	}
	return fmt.Sprintf("DISPLAY CHLAUTH('%s') TYPE(%s)", mqscQuote(spec.ChannelName), spec.RuleType), nil
}

func buildDisplayAuthorityMQSC(spec mqadmin.AuthoritySpec) (string, error) {
	if spec.Profile == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "profile is required"}
	}
	if spec.ObjectType == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "object type is required"}
	}
	if spec.Principal == "" && spec.Group == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "principal or group is required"}
	}
	if spec.Principal != "" && spec.Group != "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "specify principal or group, not both"}
	}
	parts := []string{
		fmt.Sprintf("DISPLAY AUTHREC PROFILE('%s') OBJTYPE(%s)", mqscQuote(spec.Profile), spec.ObjectType),
	}
	if spec.Principal != "" {
		parts = append(parts, fmt.Sprintf("PRINCIPAL('%s')", mqscQuote(spec.Principal)))
	} else {
		parts = append(parts, fmt.Sprintf("GROUP('%s')", mqscQuote(spec.Group)))
	}
	return strings.Join(parts, " "), nil
}

func (c *Client) runMQSCAllowNotFound(ctx context.Context, command string) error {
	err := c.RunMQSC(ctx, command)
	if err == nil {
		return nil
	}
	if errors.Is(err, mqadmin.ErrNotFound) || isMQSCNotFound(err) {
		return nil
	}
	return err
}

func isMQSCNotFound(err error) bool {
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "AMQ8147") ||
		strings.Contains(msg, "AMQ8958") ||
		strings.Contains(strings.ToLower(err.Error()), "not found")
}

func buildSetChannelAuthMQSC(spec mqadmin.ChannelAuthSpec, action string) (string, error) {
	if spec.ChannelName == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "channel name is required"}
	}
	if spec.RuleType == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "rule type is required"}
	}

	parts := []string{
		fmt.Sprintf("SET CHLAUTH('%s') TYPE(%s)", mqscQuote(spec.ChannelName), spec.RuleType),
	}
	if spec.Address != "" {
		parts = append(parts, fmt.Sprintf("ADDRESS('%s')", mqscQuote(spec.Address)))
	}
	if spec.UserSource != "" {
		parts = append(parts, fmt.Sprintf("USERSRC(%s)", spec.UserSource))
	}
	if spec.CheckClient != "" {
		parts = append(parts, fmt.Sprintf("CHCKCLNT(%s)", spec.CheckClient))
	}
	if spec.Description != "" {
		parts = append(parts, fmt.Sprintf("DESCR('%s')", mqscQuote(spec.Description)))
	}
	parts = append(parts, fmt.Sprintf("ACTION(%s)", action))
	return strings.Join(parts, " "), nil
}

func buildSetAuthorityMQSC(spec mqadmin.AuthoritySpec, remove bool) (string, error) {
	if spec.Profile == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "profile is required"}
	}
	if spec.ObjectType == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "object type is required"}
	}
	if spec.Principal == "" && spec.Group == "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "principal or group is required"}
	}
	if spec.Principal != "" && spec.Group != "" {
		return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "specify principal or group, not both"}
	}

	parts := []string{
		fmt.Sprintf("SET AUTHREC PROFILE('%s') OBJTYPE(%s)", mqscQuote(spec.Profile), spec.ObjectType),
	}
	if spec.Principal != "" {
		parts = append(parts, fmt.Sprintf("PRINCIPAL('%s')", mqscQuote(spec.Principal)))
	} else {
		parts = append(parts, fmt.Sprintf("GROUP('%s')", mqscQuote(spec.Group)))
	}
	if remove {
		parts = append(parts, "AUTHRMV(ALL)")
	} else {
		if len(spec.Authorities) == 0 {
			return "", &mqadmin.TerminalError{Reason: invalidSpecReason, Message: "authorities are required"}
		}
		authList := strings.Join(spec.Authorities, ",")
		parts = append(parts, fmt.Sprintf("AUTHADD(%s)", authList))
	}
	parts = append(parts, "ACTION(REPLACE)")
	return strings.Join(parts, " "), nil
}

func mqscQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
