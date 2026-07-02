//go:build windows

package openvpn3

// RuntimeDLLs returns the base names of the dynamic C-runtime libraries that a
// CGO_ENABLED=1 consumer binary must ship beside its executable on Windows
// (Windows resolves a process's DLLs from its exe dir first). They come from the
// MinGW UCRT64 toolchain that built the cgo engine; the bootstrap resolves the
// source directory (UCRT64\bin) — see bootstrap.RuntimeDLLSourceDir.
//
// Consumers copy these next to their .exe during release packaging. Returns the
// names on every Windows build (cgo or not) so release tooling is build-agnostic.
func RuntimeDLLs() []string {
	return []string{
		"libssl-3-x64.dll",
		"libcrypto-3-x64.dll",
		"libstdc++-6.dll",
		"libgcc_s_seh-1.dll",
		"libwinpthread-1.dll",
	}
}
