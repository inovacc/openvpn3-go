//go:build !windows

package openvpn3

// RuntimeDLLs returns nil off Windows: the Linux/macOS connectors link the
// system OpenSSL + C++ runtime, so a consumer ships no bundled libraries.
func RuntimeDLLs() []string { return nil }
