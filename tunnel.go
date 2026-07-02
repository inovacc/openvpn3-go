package openvpn3

import (
	"context"
	"errors"
)

// tunnel.go is the public, build-tag-free consumption surface of the module.
// The concrete connect/disconnect/status implementation is supplied per build:
//   - cgo && windows -> engine_windows.go (real OpenVPN3 C++ engine)
//   - cgo && linux   -> engine_unix.go    (ErrNotImplemented, v0)
//   - cgo && darwin  -> engine_darwin.go  (ErrNotImplemented, v0)
//   - !cgo           -> engine_stub.go    (ErrUnavailable)
// Every variant defines the package-level Connect/Disconnect/Status/Close funcs
// this file's Tunnel wrapper delegates to.

// Version is the module/CLI version string.
const Version = "v0.1.0"

// Tunnel state values (the StatusOutput.State string).
const (
	StateDisconnected = "disconnected"
	StateConnecting   = "connecting"
	StateConnected    = "connected"
	StateError        = "error"
)

// ErrUnavailable is returned when the cgo OpenVPN3 engine is not compiled into
// the binary (built with CGO_ENABLED=0). Build with CGO_ENABLED=1 -tags cgo on
// Windows with a C toolchain; the C/C++ deps are vendored, so no bootstrap step
// is needed — only OpenSSL must be on the toolchain path (or via CGO_*FLAGS).
var ErrUnavailable = errors.New("openvpn3: cgo engine unavailable (build CGO_ENABLED=1 -tags cgo)")

// ErrNotImplemented is returned by the Linux/macOS connectors in v0: the
// vendored C/C++ deps and per-OS file separation exist, but the tunnel connect
// path is not yet wired/verified on those platforms.
var ErrNotImplemented = errors.New("openvpn3: connector not implemented on this OS yet")

// ConnectInput carries everything the engine needs to bring up a tunnel. Set
// exactly one of ConfigPath (an on-disk .ovpn) or ConfigContent (the rendered
// profile body, zero-disk). Credentials are optional (autologin/cert profiles).
type ConnectInput struct {
	ConfigPath    string
	ConfigContent string
	Username      string
	Password      string
	KillSwitch    bool
}

// SessionHandle identifies an active tunnel. Key is the session key (the path
// for on-disk profiles, or "content:<hash>" for zero-disk connects).
type SessionHandle struct {
	Key string
}

// StatusOutput is the current tunnel state. State is one of the State* consts;
// Error carries the last fatal message when State == StateError.
type StatusOutput struct {
	State string
	Error string
}

// Tunnel is the public consumption interface. The package-level singleton
// returned by Default drives the process-wide OpenVPN3 session; consumers may
// depend on this interface for testability.
type Tunnel interface {
	// Connect brings up (or idempotently re-affirms) a tunnel. Fire-and-return:
	// CONNECTED arrives later, observable via Status.
	Connect(ctx context.Context, cfg ConnectInput) (SessionHandle, error)
	// Disconnect tears down the active tunnel (if any). Always leaves the engine
	// disconnected.
	Disconnect(h SessionHandle) error
	// Status reports the current tunnel state.
	Status() StatusOutput
	// Close tears down any active tunnel and releases engine resources.
	Close() error
}

// Default returns the process-wide Tunnel backed by the singleton engine. It
// delegates to the package-level Connect/Disconnect/Status/Close functions, so
// it honors the same per-OS / cgo build selection.
func Default() Tunnel { return pkgTunnel{} }

// pkgTunnel adapts the package-level engine funcs to the Tunnel interface.
type pkgTunnel struct{}

func (pkgTunnel) Connect(ctx context.Context, cfg ConnectInput) (SessionHandle, error) {
	return Connect(ctx, cfg)
}
func (pkgTunnel) Disconnect(h SessionHandle) error { return Disconnect(h) }
func (pkgTunnel) Status() StatusOutput             { return Status() }
func (pkgTunnel) Close() error                     { return Close() }
