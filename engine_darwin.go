//go:build cgo && darwin

package openvpn3

import (
	"context"
	"log/slog"
)

// engine_darwin.go is the macOS connector. v0 scope (approved 2026-06-30): the
// per-OS dependency bootstrap (bootstrap/bootstrap_darwin.go) and file
// separation exist, but the cgo tunnel connect path is not yet wired/verified on
// macOS, so Connect reports ErrNotImplemented. A future revision lands the real
// OpenVPN3 C++ binding for macOS (utun, Homebrew OpenSSL) here.

// SetLogger is a no-op until the macOS engine is wired.
func SetLogger(*slog.Logger) {}

// Connect reports the macOS connector is not implemented yet.
func Connect(_ context.Context, _ ConnectInput) (SessionHandle, error) {
	return SessionHandle{}, ErrNotImplemented
}

// Disconnect is a no-op (nothing was ever brought up).
func Disconnect(SessionHandle) error { return nil }

// Status always reports disconnected.
func Status() StatusOutput { return StatusOutput{State: StateDisconnected} }

// Close is a no-op.
func Close() error { return nil }

// Available reports false: the macOS connector is not implemented in v0.
func Available() bool { return false }
