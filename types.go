package openvpn3

import "errors"

// ErrOpenVPN3NotBuilt is returned by the stub when the binary was compiled
// without cgo or on a non-Windows platform.
var ErrOpenVPN3NotBuilt = errors.New("openvpn3: connector not built (build on Windows with CGO_ENABLED=1 + OpenSSL)")

// Error sentinels for the cgo path.
var (
	ErrEvalConfig = errors.New("openvpn3: eval_config failed")
	ErrConnect    = errors.New("openvpn3: connect failed")
	ErrNotRunning = errors.New("openvpn3: client not running")
)

// Config is the connection input. Content is the inline .ovpn profile text
// (OpenVPN3 requires inline profiles; use profile.go to merge external refs).
type Config struct {
	Content  string // full .ovpn content, inline
	Username string
	Password string
}

// Event is one OpenVPN3 ClientAPI event (CONNECTED, RECONNECTING, AUTH_FAILED...).
type Event struct {
	Name  string
	Info  string
	Error bool // true for error/fatal events
	Fatal bool
}

// Stats is a transport-stats snapshot.
type Stats struct {
	BytesIn  int64
	BytesOut int64
}
