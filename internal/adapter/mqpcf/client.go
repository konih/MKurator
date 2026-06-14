package mqpcf

import (
	"context"
	"fmt"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

// Config holds connection parameters for a PCF/MQI client.
// Connection details and TLS are deferred; see ADR-0017.
type Config struct {
	QueueManager string
}

// Client implements mqadmin.Admin over IBM MQ PCF.
// All methods are stubs until PCF commands are wired (ADR-0017).
type Client struct {
	queueManager string
}

// NewClient builds an mqpcf client from Config.
func NewClient(cfg Config) (*Client, error) {
	if cfg.QueueManager == "" {
		return nil, fmt.Errorf("queue manager name is required")
	}
	return &Client{queueManager: cfg.QueueManager}, nil
}

var _ mqadmin.Admin = (*Client)(nil)

func (c *Client) Ping(ctx context.Context) error {
	return errNotImplemented("Ping")
}

func (c *Client) GetQueue(ctx context.Context, spec mqadmin.QueueSpec) (*mqadmin.QueueState, error) {
	return nil, errNotImplemented("GetQueue")
}

func (c *Client) DefineQueue(ctx context.Context, spec mqadmin.QueueSpec) error {
	return errNotImplemented("DefineQueue")
}

func (c *Client) DeleteQueue(ctx context.Context, spec mqadmin.QueueSpec) error {
	return errNotImplemented("DeleteQueue")
}

func (c *Client) GetTopic(ctx context.Context, name string) (*mqadmin.TopicState, error) {
	return nil, errNotImplemented("GetTopic")
}

func (c *Client) DefineTopic(ctx context.Context, spec mqadmin.TopicSpec) error {
	return errNotImplemented("DefineTopic")
}

func (c *Client) DeleteTopic(ctx context.Context, name string) error {
	return errNotImplemented("DeleteTopic")
}

func (c *Client) GetChannel(ctx context.Context, spec mqadmin.ChannelSpec) (*mqadmin.ChannelState, error) {
	return nil, errNotImplemented("GetChannel")
}

func (c *Client) DefineChannel(ctx context.Context, spec mqadmin.ChannelSpec) error {
	return errNotImplemented("DefineChannel")
}

func (c *Client) DeleteChannel(ctx context.Context, spec mqadmin.ChannelSpec) error {
	return errNotImplemented("DeleteChannel")
}

func (c *Client) SetChannelAuth(ctx context.Context, spec mqadmin.ChannelAuthSpec) error {
	return errNotImplemented("SetChannelAuth")
}

func (c *Client) GetChannelAuth(ctx context.Context, spec mqadmin.ChannelAuthSpec) (*mqadmin.ChannelAuthState, error) {
	return nil, errNotImplemented("GetChannelAuth")
}

func (c *Client) DeleteChannelAuth(ctx context.Context, spec mqadmin.ChannelAuthSpec) error {
	return errNotImplemented("DeleteChannelAuth")
}

func (c *Client) SetAuthority(ctx context.Context, spec mqadmin.AuthoritySpec) error {
	return errNotImplemented("SetAuthority")
}

func (c *Client) GetAuthority(ctx context.Context, spec mqadmin.AuthoritySpec) (*mqadmin.AuthorityState, error) {
	return nil, errNotImplemented("GetAuthority")
}

func (c *Client) DeleteAuthority(ctx context.Context, spec mqadmin.AuthoritySpec) error {
	return errNotImplemented("DeleteAuthority")
}
