//go:build !(windows && cgo)

package openvpn3

import (
	"context"
	"errors"
	"testing"
)

func TestStub_NewReturnsNotBuilt(t *testing.T) {
	_, err := New(Config{Content: "client\n"}, nil)
	if !errors.Is(err, ErrOpenVPN3NotBuilt) {
		t.Fatalf("want ErrOpenVPN3NotBuilt, got %v", err)
	}
}

func TestStub_ConnectReturnsNotBuilt(t *testing.T) {
	var c *Client
	if err := c.Connect(context.Background()); !errors.Is(err, ErrOpenVPN3NotBuilt) {
		t.Fatalf("want ErrOpenVPN3NotBuilt, got %v", err)
	}
}
