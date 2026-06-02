package mqadmin

import (
	"context"
	"errors"
	"fmt"

	messagingv1alpha1 "github.com/konradheimel/kurator/api/v1alpha1"
)

// Factory builds Admin clients for a QueueManagerConnection.
type Factory interface {
	ForConnection(ctx context.Context, conn *messagingv1alpha1.QueueManagerConnection) (Admin, error)
}

// Admin is the seam between reconcilers and IBM MQ.
type Admin interface {
	Ping(ctx context.Context) error
	GetQueue(ctx context.Context, spec QueueSpec) (*QueueState, error)
	DefineQueue(ctx context.Context, spec QueueSpec) error
	DeleteQueue(ctx context.Context, spec QueueSpec) error
	GetTopic(ctx context.Context, name string) (*TopicState, error)
	DefineTopic(ctx context.Context, spec TopicSpec) error
	DeleteTopic(ctx context.Context, name string) error
	GetChannel(ctx context.Context, spec ChannelSpec) (*ChannelState, error)
	DefineChannel(ctx context.Context, spec ChannelSpec) error
	DeleteChannel(ctx context.Context, spec ChannelSpec) error
}

// QueueSpec is the domain shape for defining a queue via MQSC.
type QueueSpec struct {
	Name       string
	Type       QueueType
	Attributes map[string]string
}

// QueueState is the observed MQSC attributes of a queue.
type QueueState struct {
	Name       string
	Attributes map[string]string
}

// TopicSpec is the domain shape for defining a topic via MQSC.
type TopicSpec struct {
	Name       string
	Attributes map[string]string
}

// TopicState is the observed MQSC attributes of a topic.
type TopicState struct {
	Name       string
	Attributes map[string]string
}

// ChannelType mirrors supported channel kinds in the operator.
type ChannelType string

const (
	ChannelTypeSvrconn ChannelType = "svrconn"
)

// ChannelSpec is the domain shape for defining a channel via MQSC.
type ChannelSpec struct {
	Name       string
	Type       ChannelType
	Attributes map[string]string
}

// ChannelState is the observed MQSC attributes of a channel.
type ChannelState struct {
	Name       string
	Attributes map[string]string
}

// Sentinel errors for controller branching.
var (
	ErrNotFound  = errors.New("mq object not found")
	ErrTerminal  = errors.New("mq terminal error")
	ErrTransient = errors.New("mq transient error")
)

// TerminalError wraps a non-retryable MQ or REST failure.
type TerminalError struct {
	Reason  string
	Message string
	Cause   error
}

func (e *TerminalError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *TerminalError) Unwrap() error { return e.Cause }

func (e *TerminalError) Is(target error) bool { return target == ErrTerminal }

// TransientError wraps a retryable failure (5xx, timeout, QM unavailable).
type TransientError struct {
	Message string
	Cause   error
}

func (e *TransientError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *TransientError) Unwrap() error { return e.Cause }

func (e *TransientError) Is(target error) bool { return target == ErrTransient }

// NotFoundError indicates the MQ object does not exist.
type NotFoundError struct {
	Object string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("mq object %q not found", e.Object)
}

func (e *NotFoundError) Is(target error) bool { return target == ErrNotFound }
