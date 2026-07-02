//go:build !windows

package main

import (
	"fmt"
	"os"
)

// setupOpenSSL is a no-op off Windows: unix/darwin toolchains find OpenSSL on
// their default search paths (or via CGO_CPPFLAGS/CGO_LDFLAGS).
func setupOpenSSL(root string) error {
	fmt.Fprintln(os.Stderr, "openvpn: nothing to do on this OS (OpenSSL comes from default toolchain paths)")
	return nil
}
