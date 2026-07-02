//go:build windows && cgo

package openvpn3

/*
#include <stdlib.h>
#include "ovpncli_shim.hpp"
*/
import "C"

import (
	"context"
	"fmt"
	"runtime/cgo"
	"sync"
	"unsafe"
)

// msgBufLen bounds the message buffer the C shim writes eval_config / connect
// errors into. Generous; the shim NUL-terminates within this length.
const msgBufLen = 1024

// Client is the cgo-backed OpenVPN3 client. It wraps the C LensrClient handle,
// a cgo.Handle for the C user pointer (so Go pointers never cross into C — the
// cgo pointer-passing rule), the caller's EventSink, and the Windows tun
// bridge that turns TunBuilder callbacks into elevated-helper requests.
type Client struct {
	c      *C.lensr_ovpn3 // owned C client; freed in free()
	handle cgo.Handle     // resolves back to *Client inside C callbacks
	sink   EventSink
	tun    *tunBridge

	mu      sync.Mutex
	running bool
	freed   bool
}

// New constructs a client, evaluates the profile (staging credentials when a
// username is present), and wires the callback table. It does NOT connect;
// call Connect. The returned Client must be released via Stop (or a failed
// Connect) which frees the C handle and the cgo.Handle.
func New(cfg Config, sink EventSink) (*Client, error) {
	if sink == nil {
		sink = noopSink{}
	}

	cl := &Client{
		sink: sink,
		tun:  newTunBridge(),
	}
	// Register the handle BEFORE building the C callback table so any callback
	// the C side fires during eval_config can resolve *Client.
	cl.handle = cgo.NewHandle(cl)

	// newCClient (callbacks.go) wires the //export'd trampolines into the C
	// callback table via the static lensr_make_cb constructor and allocates the
	// C client. Kept there because a static C function is preamble-local.
	cl.c = newCClient(cl.handle)
	if cl.c == nil {
		cl.handle.Delete()
		return nil, fmt.Errorf("%w: lensr_ovpn3_new returned NULL (alloc failure)", ErrConnect)
	}

	if err := cl.evalConfig(cfg); err != nil {
		cl.free()
		return nil, err
	}

	return cl, nil
}

// evalConfig calls the C eval_config, surfacing any parse/credential error.
func (c *Client) evalConfig(cfg Config) error {
	content := C.CString(cfg.Content)
	defer C.free(unsafe.Pointer(content))
	user := C.CString(cfg.Username)
	defer C.free(unsafe.Pointer(user))
	pass := C.CString(cfg.Password)
	defer C.free(unsafe.Pointer(pass))

	msg := (*C.char)(C.malloc(msgBufLen))
	defer C.free(unsafe.Pointer(msg))

	rc := C.lensr_ovpn3_eval_config(c.c, content, user, pass, msg, C.size_t(msgBufLen))
	if rc != 0 {
		return fmt.Errorf("%w: %s", ErrEvalConfig, C.GoString(msg))
	}
	return nil
}

// Connect runs the blocking C connect on a dedicated goroutine and waits for it
// to return or for ctx to be cancelled. On ctx cancellation it issues an async
// Stop (safe from another thread per the ABI) and waits for the blocking call
// to unwind. Returns the connect error (clean disconnect => nil) or ctx.Err().
//
// Connect must be called at most once. After it returns the client is stopped
// and its resources are released.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.freed {
		c.mu.Unlock()
		return ErrNotRunning
	}
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("%w: connect already in progress", ErrConnect)
	}
	c.running = true
	c.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		msg := (*C.char)(C.malloc(msgBufLen))
		defer C.free(unsafe.Pointer(msg))

		// BLOCKS until stop()/error. The C side drives the TunBuilder +
		// event/log callbacks (callbacks.go) on this OS thread.
		rc := C.lensr_ovpn3_connect(c.c, msg, C.size_t(msgBufLen))
		if rc != 0 {
			done <- fmt.Errorf("%w: %s", ErrConnect, C.GoString(msg))
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		c.finish()
		return err
	case <-ctx.Done():
		// Async stop, then drain the blocking call so we never free the C
		// client while connect() is still touching it.
		C.lensr_ovpn3_stop(c.c)
		<-done
		c.finish()
		return ctx.Err()
	}
}

// Stop requests an async disconnect. If a Connect is in flight it unblocks it;
// the in-flight Connect performs the resource release. If no Connect ran, Stop
// releases resources directly. Safe to call multiple times.
func (c *Client) Stop() error {
	c.mu.Lock()
	if c.freed {
		c.mu.Unlock()
		return nil
	}
	running := c.running
	cptr := c.c
	c.mu.Unlock()

	if cptr != nil {
		C.lensr_ovpn3_stop(cptr)
	}
	if !running {
		// No Connect goroutine to do cleanup; release here.
		c.finish()
	}
	return nil
}

// TransportStats snapshots the byte counters. Errors if the client is gone.
func (c *Client) TransportStats() (Stats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.freed || c.c == nil {
		return Stats{}, ErrNotRunning
	}

	var in, out C.longlong
	rc := C.lensr_ovpn3_transport_stats(c.c, &in, &out)
	if rc != 0 {
		return Stats{}, ErrNotRunning
	}
	return Stats{BytesIn: int64(in), BytesOut: int64(out)}, nil
}

// finish marks the connect as no longer running and releases resources once.
func (c *Client) finish() {
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()
	c.free()
}

// free releases the C client and the cgo.Handle exactly once. Idempotent.
func (c *Client) free() {
	c.mu.Lock()
	if c.freed {
		c.mu.Unlock()
		return
	}
	c.freed = true
	cptr := c.c
	c.c = nil
	h := c.handle
	c.mu.Unlock()

	if cptr != nil {
		C.lensr_ovpn3_free(cptr)
	}
	h.Delete()
}

// noopSink is used when the caller passes a nil EventSink.
type noopSink struct{}

func (noopSink) OnEvent(Event) {}
func (noopSink) OnLog(string)  {}
