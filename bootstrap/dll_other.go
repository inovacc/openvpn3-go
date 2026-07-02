//go:build !windows

package bootstrap

// RuntimeDLLSourceDir returns "" off Windows: the Linux/macOS connectors link
// the system OpenSSL + C++ runtime, so there are no bundled DLLs to copy.
func RuntimeDLLSourceDir() string { return "" }
