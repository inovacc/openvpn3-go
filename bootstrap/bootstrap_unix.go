//go:build linux

package bootstrap

import "os/exec"

// bootstrapOS performs the Linux-specific bootstrap. The common C/C++ deps
// (asio/lz4/jsoncpp) + the OpenVPN3 core are already fetched by Run. v0 scope:
// the Linux cgo connector is not yet wired (engine_unix.go reports
// ErrNotImplemented), so this only verifies the system OpenSSL is discoverable
// for when the real binding lands. No generated cgo flags are written (system
// OpenSSL resolves on the default search path / via pkg-config).
func bootstrapOS(_ string, opts Options) error {
	if _, err := exec.LookPath("pkg-config"); err == nil {
		if err := run("", "pkg-config", "--exists", "openssl"); err == nil {
			opts.logf("openvpn: system OpenSSL found (pkg-config)")
		} else {
			opts.logf("WARN: OpenSSL dev package not found (install libssl-dev) for the future Linux connector")
		}
	} else {
		opts.logf("openvpn: pkg-config absent; ensure libssl-dev is installed for the future Linux connector")
	}
	opts.logf("openvpn: deps ready (Linux connector is ErrNotImplemented in v0)")
	return nil
}
