//go:build darwin

package bootstrap

import (
	"os/exec"
	"strings"
)

// bootstrapOS performs the macOS-specific bootstrap. The common C/C++ deps +
// OpenVPN3 core are already fetched by Run. v0 scope: the macOS cgo connector is
// not yet wired (engine_darwin.go reports ErrNotImplemented), so this only
// locates the Homebrew OpenSSL prefix for when the real binding lands.
func bootstrapOS(_ string, opts Options) error {
	out, err := exec.Command("brew", "--prefix", "openssl@3").Output()
	if err == nil {
		opts.logf("openvpn: Homebrew OpenSSL at %s", strings.TrimSpace(string(out)))
	} else {
		opts.logf("WARN: brew openssl@3 not found (brew install openssl@3) for the future macOS connector")
	}
	opts.logf("openvpn: deps ready (macOS connector is ErrNotImplemented in v0)")
	return nil
}
