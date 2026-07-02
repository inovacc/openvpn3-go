//go:build cgo && windows

package openvpn3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// engine_windows.go is the real in-process OpenVPN3 connector. It drives the
// low-level cgo Client (client.go) through a fire-and-return lifecycle against a
// process-wide singleton: Connect launches the blocking C connect on a
// goroutine and returns immediately; CONNECTED arrives later via the sink
// (observable through Status); Disconnect stops the client and drains.

// disconnectDrainTimeout bounds how long Disconnect waits for the blocking
// Connect goroutine to unwind after Stop() before returning anyway.
const disconnectDrainTimeout = 10 * time.Second

// engine holds the mutable state of the single in-process VPN session. Every
// field read/write is guarded by mu except the atomics, which are set from the
// sink callbacks (running on a cgo-attached, non-Go-created thread).
type engine struct {
	logger *slog.Logger

	mu          sync.Mutex
	client      *Client    // nil until connect; nil again after teardown
	connectDone chan error // buffered(1); receives Client.Connect's return
	configPath  string     // current session key (path, or "content:<hash>"); guarded by mu
	lastContent string     // resolved .ovpn text of the current session; guarded by mu

	connected atomic.Bool // set true on CONNECTED event, false on teardown
	fatal     atomic.Bool // set true on a fatal event
	lastErr   atomic.Pointer[string]
}

// defaultEngine is the process-wide singleton — the host owns exactly one
// OpenVPN3 session.
var defaultEngine = &engine{}

// SetLogger installs a logger on the default engine (optional; defaults to
// slog.Default()).
func SetLogger(l *slog.Logger) { defaultEngine.logger = l }

func (s *engine) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}

	return slog.Default()
}

// Connect brings up (or idempotently re-affirms) a tunnel for the given profile.
func Connect(ctx context.Context, cfg ConnectInput) (SessionHandle, error) {
	return defaultEngine.connect(ctx, cfg)
}

// Disconnect stops the live client (if any) and waits, bounded, for the Connect
// goroutine to unwind. It always leaves the engine disconnected.
func Disconnect(h SessionHandle) error { return defaultEngine.disconnect(h) }

// Status computes the current state from the flags and live client.
func Status() StatusOutput { return defaultEngine.status() }

// Close tears down any active tunnel and leaves the engine disconnected.
func Close() error { return defaultEngine.disconnect(SessionHandle{}) }

func (s *engine) connect(ctx context.Context, cfg ConnectInput) (SessionHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Session key (cheap, no I/O): the path for on-disk profiles, or a short hash
	// of the in-memory content for zero-disk connects.
	sessionKey := cfg.ConfigPath
	if cfg.ConfigContent != "" {
		sum := sha256.Sum256([]byte(cfg.ConfigContent))
		sessionKey = "content:" + hex.EncodeToString(sum[:8])
	}

	if s.client != nil {
		if sessionKey == s.configPath {
			return SessionHandle{Key: sessionKey}, nil
		}

		s.log().Info("openvpn3: switching profile", "from", s.configPath, "to", sessionKey)
		s.teardownLocked()
	}

	// Zero-disk: prefer the in-memory ConfigContent. Otherwise read the path
	// (os.ReadFile runs with the elevated host token).
	content := cfg.ConfigContent
	if content == "" {
		b, err := os.ReadFile(cfg.ConfigPath)
		if err != nil {
			s.log().Error("openvpn3: read profile failed", "err", err)

			return SessionHandle{}, fmt.Errorf("read profile %q: %w", cfg.ConfigPath, err)
		}

		content = string(b)
	}

	core, err := ConfigFromOVPN(content, Credentials{User: cfg.Username, Pass: cfg.Password})
	if err != nil {
		s.log().Error("openvpn3: build config failed", "err", err)

		return SessionHandle{}, fmt.Errorf("build config: %w", err)
	}

	sink := &engineSink{eng: s}

	client, err := New(core, sink)
	if err != nil {
		s.log().Error("openvpn3: new client failed", "err", err)

		return SessionHandle{}, fmt.Errorf("new client: %w", err)
	}

	if cfg.KillSwitch {
		s.log().Warn("openvpn3: WFP kill-switch requested but not implemented in v0")
	}

	s.client = client
	s.configPath = sessionKey
	s.lastContent = content
	s.connectDone = make(chan error, 1)
	s.connected.Store(false)
	s.fatal.Store(false)
	s.lastErr.Store(nil)

	done := s.connectDone

	go func() {
		// BLOCKS until session end / Stop. Client.Connect self-releases the C
		// handle on return, so we must NOT call client.free here.
		done <- client.Connect(ctx)
	}()

	return SessionHandle{Key: sessionKey}, nil
}

func (s *engine) status() StatusOutput {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := StatusOutput{State: s.stateLocked()}
	if s.fatal.Load() {
		if msg := s.lastErr.Load(); msg != nil {
			out.Error = *msg
		}
	}

	return out
}

func (s *engine) disconnect(_ SessionHandle) error {
	s.mu.Lock()
	client := s.client
	done := s.connectDone
	s.mu.Unlock()

	if client == nil {
		return nil
	}

	if err := client.Stop(); err != nil {
		s.log().Warn("openvpn3: client.Stop error", "err", err)
	}

	if done != nil {
		select {
		case err := <-done:
			if err != nil {
				s.log().Info("openvpn3: connect goroutine ended", "err", err)
			}
		case <-time.After(disconnectDrainTimeout):
			s.log().Warn("openvpn3: disconnect drain timed out", "timeout", disconnectDrainTimeout)
		}
	}

	s.mu.Lock()
	s.client = nil
	s.connectDone = nil
	s.configPath = ""
	s.lastContent = ""
	s.connected.Store(false)
	s.mu.Unlock()

	return nil
}

// stateLocked derives the state string from the flags + client presence. Caller
// must hold s.mu.
func (s *engine) stateLocked() string {
	switch {
	case s.fatal.Load():
		return StateError
	case s.connected.Load():
		return StateConnected
	case s.client != nil:
		return StateConnecting
	default:
		return StateDisconnected
	}
}

// teardownLocked stops the current client and drains the Connect goroutine.
// Caller MUST hold s.mu.
func (s *engine) teardownLocked() {
	client := s.client
	done := s.connectDone

	s.client = nil
	s.connectDone = nil
	s.configPath = ""
	s.lastContent = ""
	s.connected.Store(false)
	s.fatal.Store(false)
	s.lastErr.Store(nil)

	if client == nil {
		return
	}

	_ = client.Stop()

	// Release the lock while draining so the Connect goroutine is not starved.
	s.mu.Unlock()

	if done != nil {
		select {
		case <-done:
		case <-time.After(disconnectDrainTimeout):
			s.log().Warn("openvpn3: teardown drain timed out", "timeout", disconnectDrainTimeout)
		}
	}

	s.mu.Lock()
}

// engineSink adapts low-level events/logs onto slog and the engine's atomic
// state flags. Both methods may run on a cgo-attached, non-Go-created thread, so
// they only touch slog + atomics and return promptly (never take s.mu).
type engineSink struct {
	eng *engine
}

func (k *engineSink) OnEvent(e Event) {
	k.eng.log().Info("openvpn3 event",
		"name", e.Name,
		"info", e.Info,
		"error", e.Error,
		"fatal", e.Fatal)

	if e.Name == "CONNECTED" {
		k.eng.connected.Store(true)
	}

	if e.Fatal {
		k.eng.fatal.Store(true)

		msg := e.Name
		if e.Info != "" {
			msg = e.Name + ": " + e.Info
		}

		k.eng.lastErr.Store(&msg)
	}

	// Fan to the registered observer (CLI / library consumer), if any.
	emitEvent(e)
}

func (k *engineSink) OnLog(s string) {
	k.eng.log().Debug("openvpn3", "log", s)

	emitLog(s)
}

// Available reports whether the real cgo OpenVPN3 engine is compiled into this
// build (true on cgo && windows).
func Available() bool { return true }
