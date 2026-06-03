package mqadmin

import (
	"errors"
	"fmt"
	"testing"
)

func TestTerminalErrorWithoutCause(t *testing.T) {
	t.Parallel()
	err := &TerminalError{Reason: "Auth", Message: "denied"}
	if err.Error() != "denied" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestTransientErrorWithoutCause(t *testing.T) {
	t.Parallel()
	err := &TransientError{Message: "timeout"}
	if err.Error() != "timeout" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestTerminalError(t *testing.T) {
	t.Parallel()
	cause := fmt.Errorf("root")
	err := &TerminalError{Reason: "Auth", Message: "denied", Cause: cause}
	if err.Error() != "denied: root" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if !errors.Is(err, ErrTerminal) {
		t.Fatal("expected ErrTerminal")
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected unwrap to cause")
	}
}

func TestTransientError(t *testing.T) {
	t.Parallel()
	err := &TransientError{Message: "timeout"}
	if !errors.Is(err, ErrTransient) {
		t.Fatal("expected ErrTransient")
	}
}

func TestTransientErrorWithCause(t *testing.T) {
	t.Parallel()
	cause := fmt.Errorf("dial tcp: timeout")
	err := &TransientError{Message: "mqweb unreachable", Cause: cause}
	if err.Error() != "mqweb unreachable: dial tcp: timeout" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected unwrap to cause")
	}
}

func TestNotFoundError(t *testing.T) {
	t.Parallel()
	err := &NotFoundError{Object: "APP.X"}
	if err.Error() != `mq object "APP.X" not found` {
		t.Fatalf("Error() = %q", err.Error())
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}
