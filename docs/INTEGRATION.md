# Incremental Integration Guide

<!-- rev:001 -->

How to bring the already-working OpenVPN3 **Go + C/C++** set into this scaffold
one increment at a time, keeping the tree green at every step.

## Invariants (never break these)

1. **`pkg/ovpn3` public API (`Client`, `Config`, `Status`, `Event`) stays
   stable.** Everything else hangs off it. Extend, don't reshape.
2. **`CGO_ENABLED=0 go build ./...` always passes.** The stub
   (`client_stub.go`) is the safety net for CI and non-C platforms.
3. **Validation and hardening live in build-tag-free files** (`client.go`,
   `errors.go`) so they run on every build and under `go test -short`.
4. **No secret hits a log.** Credentials are copied, used, then `Wipe`d.

## Module map

| Path | Role |
|------|------|
| `pkg/ovpn3/client.go` | public contract + all input validation/hardening |
| `pkg/ovpn3/errors.go` | sentinel errors (match with `errors.Is`) |
| `pkg/ovpn3/client_stub.go` (`!cgo`) | pure-Go fallback → `ErrUnsupported` |
| `pkg/ovpn3/client_cgo.go` (`cgo`) | the integration seam → native ClientAPI |
| `pkg/ovpn3/native/` | drop-zone for C/C++ headers, sources, libs |
| `internal/vpnsvc/service.go` | adapts `ovpn3.Client` into daemon Start/Stop |

## Increment order

### 1 — Land the native sources

Drop the working C/C++ into `pkg/ovpn3/native/` (`include/`, `src/`, `lib/`).
List the public headers and the single bridge entry point you will call from
Go (a C-ABI shim over the C++ ClientAPI — cgo cannot call C++ directly).

### 2 — Wire the cgo bridge

In `client_cgo.go`, replace the placeholder body with the real preamble:

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

Move your existing Go code into `internal/` packages (e.g. profile parsing,
transport helpers), consumed *through* `pkg/ovpn3`. Keep the public API stable;
adapt callers to it rather than widening it.

### 5 — Feed real config into the daemon

Extend `internal/app/app.go` config with the VPN profile/credentials, then
build an `ovpn3.Config` in `main.go`'s `core` instead of the empty placeholder.

## Per-increment hardening checklist

- [ ] New input validated in `Config.validate` (or a sibling) before any native call
- [ ] Native call wrapped in `safeCall`; no panic escapes
- [ ] Resources released on every path (`defer`); `Close` idempotent
- [ ] Credentials copied + `Wipe`d; never logged
- [ ] Sentinel error added + matched with `errors.Is`; lowercase, no trailing punct
- [ ] Table test added; `go test -short ./...` green
- [ ] Both `CGO_ENABLED=0` and `=1` builds pass
- [ ] `go vet ./...` and `golangci-lint run` clean
