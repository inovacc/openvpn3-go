# Incremental Integration Guide

<!-- rev:002 -->

How the OpenVPN3 **Go + C/C++** set is integrated in this module: all C/C++
build dependencies (OpenVPN3 core, asio, lz4, jsoncpp, tap-windows.h) are
vendored in-repo under `openvpn/` and `deps/`, so integration work happens
directly against the vendored tree — no separate "bring the sources in" step.
This guide keeps the tree green at every step.

## Invariants (never break these)

1. **The root `openvpn3` package public API (`Tunnel`, `Config`, `ConnectInput`,
   `Status`, `Event`) stays stable.** Everything else hangs off it. Extend,
   don't reshape.
2. **`CGO_ENABLED=0 go build ./...` always passes.** The stub
   (`engine_stub.go`) is the safety net for CI and non-C platforms.
3. **Validation and hardening live in build-tag-free files** (`client.go`,
   `errors.go`) so they run on every build and under `go test -short`.
4. **No secret hits a log.** Credentials are copied, used, then `Wipe`d.

## Module map

| Path | Role |
|------|------|
| `client.go` | public contract + all input validation/hardening |
| `tunnel.go` | build-tag-free public surface (sentinel errors, `Tunnel` wrapper) |
| `engine_stub.go` (`!cgo`) | pure-Go fallback → `ErrUnavailable` |
| `engine_windows.go`, `cgo_flags.go` (`cgo && windows`) | the integration seam → native ClientAPI |
| `openvpn/`, `deps/` | vendored C/C++ headers, sources (OpenVPN3 core, asio, lz4, jsoncpp, tap-windows.h) |
| `cmd/openvpn` | CLI consumer of the `openvpn3` package |

## Increment order

### 1 — Native sources (done)

The working C/C++ lives vendored under `openvpn/` (OpenVPN3 core) and `deps/`
(asio, lz4, jsoncpp, tap-windows.h) — no drop-zone step. The public headers and
the C-ABI shim entry point (cgo cannot call C++ directly) are under `shim/`.

### 2 — Wire the cgo bridge

In `cgo_flags.go` / `engine_windows.go`, the real preamble is already wired:

```go
/*
#cgo CXXFLAGS: -std=c++17 -I${SRCDIR}/native/include
#cgo LDFLAGS: -L${SRCDIR}/native/lib -lovpn3core -lstdc++
#include "ovpn3_cgo.h"
*/
import "C"
```

Verify: `CGO_ENABLED=1 go build ./...` (needs gcc/clang). The `CGO_ENABLED=0`
build must still pass via the stub.

### 3 — Map ClientAPI → facade, one method at a time

For each method (`Connect`, `Disconnect`, `Status`, events), bridge the native
call. **Wrap every native call in `safeCall`** so a C++ exception crossing the
boundary becomes `ErrNativePanic`, never a process crash. Add a table-test for
each before moving on.

### 4 — Migrate existing Go incrementally

Keep profile parsing and transport helpers at the flat package root
(`profile.go`, `tunspec.go`), consumed *through* the `openvpn3` package's
public surface. Keep the public API stable; adapt callers to it rather than
widening it.

### 5 — Feed real config into the CLI

Extend `cmd/openvpn`'s config with the VPN profile/credentials, then build a
real `openvpn3.ConnectInput` / `Config` in `main.go` instead of the empty
placeholder.

## Per-increment hardening checklist

- [ ] New input validated in `Config.validate` (or a sibling) before any native call
- [ ] Native call wrapped in `safeCall`; no panic escapes
- [ ] Resources released on every path (`defer`); `Close` idempotent
- [ ] Credentials copied + `Wipe`d; never logged
- [ ] Sentinel error added + matched with `errors.Is`; lowercase, no trailing punct
- [ ] Table test added; `go test -short ./...` green
- [ ] Both `CGO_ENABLED=0` and `=1` builds pass
- [ ] `go vet ./...` and `golangci-lint run` clean
