//go:build !(windows && cgo)

package openvpn3

import "context"

// Client is a no-op stub when built without the "openvpn3" tag.
type Client struct{}

// EventSink is declared in sink.go (untagged) so it is shared by both builds.

// New always fails in the stub build.
func New(_ Config, _ EventSink) (*Client, error) { return nil, ErrOpenVPN3NotBuilt }

func (c *Client) Connect(_ context.Context) error { return ErrOpenVPN3NotBuilt }
func (c *Client) Stop() error                     { return ErrOpenVPN3NotBuilt }
func (c *Client) TransportStats() (Stats, error)  { return Stats{}, ErrOpenVPN3NotBuilt }
