# OpenVPN3 cgo binding — C/C++ build requirements

What the cgo build of this package needs. Build tag gate:
`//go:build windows && cgo` (the complement is a pure-Go stub; no -tags needed).
Authoritative sources: `cgo_flags.go` (`#cgo` directives), `scripts/openvpn3/fetch-deps.ps1`, `.gitmodules`.

**Status:** `openvpn3` is the **default** VPN connector (ADR-0021). On a build
without this cgo connector the default resolves to the pure-Go `native` connector
(`autoModeConnector` / `openvpn3Compiled == false`), so a pure-Go binary is never
broken. The release pipeline builds the cgo `openvpn3` artifact **opt-in** —
`.goreleaser.yml` skips it unless `LENSR_BUILD_OPENVPN3=1`, with the OpenSSL CGO
flags below supplied by the build environment.

## Tools

| Tool | Purpose | Notes |
|---|---|---|
| Go (cgo) | builds the binding | `CGO_ENABLED=1` |
| MinGW **gcc** + **g++** | C/C++ compile + link | C++17 (`-std=c++17 -fexceptions`); cgo links via gcc, so `-lstdc++` is added explicitly |
| **git** | OpenVPN3 submodule + dep clones | required on PATH |
| **PowerShell** | runs `fetch-deps.ps1` | Windows |

Platform: **Windows only** (the binding is `*_windows`; uses WFP/TUN/Win32 APIs).

## Libraries

### OpenSSL (crypto/SSL backend — `-DUSE_OPENSSL`)
- Links `-lssl -lcrypto` (static `libssl.a` / `libcrypto.a` + headers).
- **Machine-specific path, not hardcoded.** On this host the MinGW *gcc* package is a
  separate scoop install from the *mingw* package that ships OpenSSL dev
  headers/libs (`opt/include` + `opt/lib`). Point cgo at them via env at build time:
  ```
  $ssl = "C:\Users\dyamm\scoop\apps\mingw\current\opt"   # has include\openssl\ + lib\libssl.a
  $env:CGO_CPPFLAGS = "-I$ssl\include"
  $env:CGO_LDFLAGS  = "-L$ssl\lib"
  ```
  A unified toolchain (e.g. MSYS2 mingw64) with OpenSSL on the default path needs no env.
- mbedTLS is **not** used (only needed for `-DUSE_MBEDTLS`).

### Windows system libraries (linked automatically)
`ws2_32` (sockets), `iphlpapi` (interface/route), `crypt32` (cert store),
`fwpuclnt` (WFP kill-switch), `ole32`, `oleaut32`, `advapi32`, `gdi32`, `user32`,
`bcrypt` (RNG), `wininet`, `setupapi`, `wtsapi32`, `rpcrt4` (UuidCreate),
`uuid` (GUID_DEVCLASS_NET), `version`. Plus `-lstdc++` (C++ runtime).

## Vendored C/C++ dependencies (`deps/` — gitignored, fetched on demand)

`scripts/openvpn3/fetch-deps.ps1` populates `pkg/openvpn3/deps/` (pinned, mirroring
`openvpn/deps/lib-versions`). Idempotent; `-Force` re-clones. Never committed.

| Dep | Version | Source | Layout used |
|---|---|---|---|
| **asio** | `asio-1-24-0` | chriskohlhoff/asio | header-only, `asio/include/asio.hpp` (`-DUSE_ASIO -DASIO_STANDALONE`, no Boost) |
| **lz4** | `v1.8.3` | lz4/lz4 | `lib/lz4.h` + `lib/lz4.c` (`-DHAVE_LZ4`; compiled in-package, no `-llz4`) |
| **jsoncpp** | `1.8.4` | open-source-parsers/jsoncpp | `include/json/json.h` |
| **tap-windows.h** | (head) | OpenVPN/tap-windows6 | single header `tap-windows/include/tap-windows.h` |

### OpenVPN3 core (git submodule)
- `pkg/openvpn3/openvpn` → `github.com/OpenVPN/openvpn3` @ **`release/3.11.6`** (`.gitmodules`).
- Init it before building: `git submodule update --init --recursive pkg/openvpn3/openvpn`.
- License: **MPL-2.0** (elected; dual AGPL-3.0/MPL-2.0 upstream) — see `NOTICE.md`.

## Compiled translation units (in this package dir only)
cgo compiles C/C++ TUs that live **directly** in `pkg/openvpn3/` (never subdirs):
- `cgo_bridge.cpp` — the single TU; `#include`s the shim (`shim/`) + the OpenVPN3
  single-TU core (`openvpn/client/ovpncli.cpp`).
- `cgo_lz4.c` — `#include`s `deps/lz4/lib/lz4.c`.
- `cgo_core_epoch_windows.cpp` — `#include`s `openvpn/crypto/data_epoch.cpp`
  (symbols `ovpncli.cpp`'s single-TU doesn't emit).

## Preprocessor defines (`cgo_flags.go`)
`USE_OPENSSL`, `USE_ASIO`, `ASIO_STANDALONE`, `HAVE_LZ4`,
`OPENVPN_CORE_API_VISIBILITY_HIDDEN`, `_WIN32_WINNT=0x0601`,
`WIN32_LEAN_AND_MEAN`, `NOMINMAX`.

## Build (end to end)
```powershell
# 1. OpenVPN3 core submodule
git submodule update --init --recursive pkg/openvpn3/openvpn
# 2. vendor asio / lz4 / jsoncpp / tap-windows.h
powershell -File scripts/openvpn3/fetch-deps.ps1
# 3. point cgo at MinGW OpenSSL (see above), then build
$env:CGO_ENABLED = 1
$ssl = "C:\Users\dyamm\scoop\apps\mingw\current\opt"
$env:CGO_CPPFLAGS = "-I$ssl\include"; $env:CGO_LDFLAGS = "-L$ssl\lib"
go build -o dist/lensr-openvpn3.exe ./cmd/lensr
# or: task build:openvpn3
```
Result is a single self-contained ~290 MB binary (statically links OpenVPN3 + OpenSSL).

For a release build, set `LENSR_BUILD_OPENVPN3=1` plus `CGO_CPPFLAGS`/`CGO_LDFLAGS`
(above) in the environment and run goreleaser; it emits a separate
`lensr_openvpn3_windows_amd64` archive alongside the pure-Go artifacts.
