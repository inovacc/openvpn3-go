//go:build windows && cgo

package openvpn3

import (
	"context"
	"os"
	"testing"
	"time"
)

// chanSink captures CONNECTED on a channel so TestLiveConnect can block until the
// tunnel reports up (or the 60s ctx fires). OnEvent runs on a cgo-attached thread,
// so it MUST NOT block: the send is non-blocking (buffered channel + default).
type chanSink struct {
	connected chan struct{}
}

func (s *chanSink) OnEvent(e Event) {
	if e.Name == "CONNECTED" {
		select {
		case s.connected <- struct{}{}:
		default:
		}
	}
}

func (s *chanSink) OnLog(string) {}

// TestLiveConnect is a gated, opt-in end-to-end check against a REAL OpenVPN
// server. It is skipped under -short and skipped unless LENSR_OVPN3_PROFILE points
// at a readable inline .ovpn profile. Given that profile it constructs a client,
// connects with a 60s deadline, waits for a CONNECTED event, then Stops.
//
// NOTE (Path-A architecture): the live connect path goes through the elevated
// __openvpn3 helper subprocess, which is launched via the UAC re-exec mechanism.
// This test therefore requires that the running account can UAC-elevate (i.e. it
// must not be a standard user with elevation fully blocked by policy). The helper
// subprocess is started automatically by Connect; no manual setup is needed beyond
// setting LENSR_OVPN3_PROFILE to an accessible .ovpn profile path.
func TestLiveConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("live connect skipped in -short mode")
	}

	profilePath := os.Getenv("LENSR_OVPN3_PROFILE")
	if profilePath == "" {
		t.Skip("LENSR_OVPN3_PROFILE unset; skipping live connect")
	}

	content, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read LENSR_OVPN3_PROFILE %q: %v", profilePath, err)
	}

	cfg, err := ConfigFromOVPN(string(content), Credentials{})
	if err != nil {
		t.Fatalf("ConfigFromOVPN: %v", err)
	}

	sink := &chanSink{connected: make(chan struct{}, 1)}

	client, err := New(cfg, sink)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Stop: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	connectErr := make(chan error, 1)
	go func() { connectErr <- client.Connect(ctx) }()

	select {
	case <-sink.connected:
		t.Log("received CONNECTED event")
	case err := <-connectErr:
		// Connect returned before CONNECTED — surface the failure.
		t.Fatalf("Connect returned before CONNECTED: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for CONNECTED: %v", ctx.Err())
	}
}
