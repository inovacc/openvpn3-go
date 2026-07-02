//go:build !cgo

package openvpn3

import (
	"context"
	"log/slog"
)

// engine_stub.go is the CGO_ENABLED=0 build. The OpenVPN3 C++ engine is absent,
// so Connect reports ErrUnavailable and the rest are benign no-ops. This keeps
// consumers compiling without the cgo toolchain (the lensr daemon defaults to
// CGO_ENABLED=0).

// SetLogger is a no-op without the engine.
func SetLogger(*slog.Logger) {}

// Connect reports that the cgo engine is not built in.
func Connect(_ context.Context, _ ConnectInput) (SessionHandle, error) {
	return SessionHandle{}, ErrUnavailable
}

// Disconnect is a no-op (nothing was ever brought up).
func Disconnect(SessionHandle) error { return nil }

// Status always reports disconnected.
func Status() StatusOutput { return StatusOutput{State: StateDisconnected} }

// Close is a no-op.
func Close() error { return nil }

// Available reports false: the cgo engine is not compiled into this build.
func Available() bool { return false }
