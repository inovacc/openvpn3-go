package openvpn3

import (
	"errors"
	"testing"
)

func TestErrOpenVPN3NotBuilt_IsSentinel(t *testing.T) {
	if ErrOpenVPN3NotBuilt == nil {
		t.Fatal("ErrOpenVPN3NotBuilt must be a non-nil sentinel")
	}

	wrapped := errors.New("ctx: " + ErrOpenVPN3NotBuilt.Error())
	_ = wrapped

	if got := ErrOpenVPN3NotBuilt.Error(); got == "" {
		t.Fatal("sentinel must have a message")
	}
}

func TestEvent_Zero(t *testing.T) {
	var e Event
	if e.Name != "" || e.Error {
		t.Fatalf("zero Event should be empty, got %+v", e)
	}
}
