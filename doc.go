// Package openvpn3 embeds the OpenVPN3 C++ core (git submodule at
// pkg/openvpn3/openvpn) via a cgo binding to provide a Go-only VPN connector.
//
// The cgo implementation is gated behind the "openvpn3" build tag. Without it,
// a stub is compiled whose entrypoints return ErrOpenVPN3NotBuilt.
//
// Build the cgo variant with: task build:openvpn3  (Windows, MinGW or MSVC, CGO_ENABLED=1).
//
// # Architecture — Path A: elevated __openvpn3 helper (Windows)
//
// On Windows, lensr uses OpenVPN3 3.11.6's NATIVE TunWin support. The cgo
// binding runs inside an elevated `__openvpn3` subprocess that is launched via
// the existing UAC re-exec mechanism and communicates with the parent lensr
// process over a named pipe (\\.\pipe\lensr_openvpn3_helper).
//
// Responsibility split:
//
//   - OpenVPN3 core owns: TUN adapter creation, IP address assignment, route
//     injection, DNS configuration, and all packet I/O.
//   - lensr owns: subprocess lifecycle management, credential handoff over the
//     pipe, status/event forwarding to the parent, and (opt-in, OFF in v0)
//     WFP kill-switch activation.
//
// WHY TunBuilder is not used on Windows: USE_TUN_BUILDER is an Android/iOS-only
// compile-time flag in OpenVPN3 3.11.6 — it is never set on Windows builds, so
// TunBuilderBase callbacks are never invoked by the Windows OpenVPN3 path.
//
// # Building the cgo connector
//
// Toolchain: Windows with a C/C++ compiler — MinGW gcc/g++ or MSVC — and
// CGO_ENABLED=1.
//
//  1. Set CGO_ENABLED=1 in the environment.
//
//  2. Run scripts/openvpn3/fetch-deps.ps1 to vendor asio, lz4 and jsoncpp into
//     pkg/openvpn3/deps/ (gitignored). `task build:openvpn3` does this for you.
//
//  3. Ensure OpenSSL dev headers/libs are visible to the C toolchain. If they are
//     not on the default path, export them before building, e.g. (PowerShell):
//
//     $env:CGO_CPPFLAGS = "-I<mingw>/opt/include"
//     $env:CGO_LDFLAGS  = "-L<mingw>/opt/lib"
//
//     On this host OpenSSL ships with the scoop mingw package under its opt/
//     directory (typically C:\Users\<user>\scoop\apps\mingw\current\opt\).
//
//  4. Build: task build:openvpn3   (or: CGO_ENABLED=1 go build -tags openvpn3 .)
//
// Run the tagged tests with: task test:openvpn3.
package openvpn3
