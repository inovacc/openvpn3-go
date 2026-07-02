# Design: CI test matrix + README badges

**Date:** 2026-07-02
**Status:** Approved (design questions defaulted to recommended options; user AFK)

## Goal

Replace the two trivial ubuntu-only workflows with one matrix `test.yml` that
exercises every build configuration of this cgo module, including the real
Windows cgo engine on a clean runner. Add status/reference badges to the README.

## Workflow: `.github/workflows/test.yml` (reshape existing; delete `build.yml`)

Keeping the file named `test.yml` preserves the badge URL and makes the badge
reflect the full matrix. `release.yaml` is untouched.

Only the `windows && cgo` files import "C" (verified: `cgo_flags.go`,
`callbacks.go`, `client.go`, `cgo_openssl_local.go`). `engine_unix.go`
(`cgo && linux`) and `engine_darwin.go` (`cgo && darwin`) are pure-Go
`ErrNotImplemented` stubs. So every matrix cell except Windows+cgo needs only the
Go toolchain.

### Job `test` — portable matrix
- `strategy.fail-fast: false`
- `matrix.os: [ubuntu-latest, macos-latest, windows-latest]`
- `matrix.cgo: ["0", "1"]`
- `exclude: {os: windows-latest, cgo: "1"}` (handled by `windows-cgo`)
- Steps: checkout → setup-go 1.25 → `go build ./...` → `go test ./...` →
  `go -C tools test ./...`, with `CGO_ENABLED: ${{ matrix.cgo }}`.
- 5 cells. Covers the stub on all three OSes + the cgo-non-Windows engine files.

### Job `windows-cgo` — the real vendored engine
- `runs-on: windows-latest`
- `msys2/setup-msys2@v2` with `msystem: UCRT64`, installing
  `mingw-w64-ucrt-x86_64-gcc` + `mingw-w64-ucrt-x86_64-openssl`.
- Resolve the UCRT64 dir location-independently via the msys2 shell
  (`cygpath -w /ucrt64` → `OPENVPN3_SSLDIR`; `cygpath -w /ucrt64/bin` →
  `GITHUB_PATH`), so it works regardless of where the action installs MSYS2.
- `go run ./cmd/openvpn bootstrap` — dogfoods the dev helper; its
  `resolveOpenSSL` reads `OPENVPN3_SSLDIR` first and writes
  `cgo_openssl_local.go`.
- `CGO_ENABLED=1 go build ./...` then `CGO_ENABLED=1 go test -short ./...`
  (`-short` skips `live_integration_test.go`, which needs a real VPN + elevation).
- Independently reproduces Task 7's local proof on a clean CI runner.

## Badges: top of `README.md` (after H1/rev line; bump `rev:001` → `rev:002`)

1. test workflow → `.../actions/workflows/test.yml`
2. Go Reference → `pkg.go.dev/badge/github.com/inovacc/openvpn3-go.svg`
3. Go Report Card → `goreportcard.com/badge/github.com/inovacc/openvpn3-go`
4. License BSD-3-Clause → links to `LICENSE`
5. Go version → `img.shields.io/github/go-mod/go-version/inovacc/openvpn3-go`

No release/version badge yet (no tags; would render "no release").

## Excluded (YAGNI)
- A `go-version` matrix axis (module pins go 1.25.0).
- Running the live integration test in CI.

## Verification
- Locally (Windows): `CGO_ENABLED=0 go build/test`, `go -C tools test ./...`, and
  the Windows cgo build all pass. Non-Windows cells and the exact runner wiring
  are validated by the first real GitHub Actions run; iterate if the msys2/gcc
  wiring needs a tweak.

## Delivery
Committed to the open branch `feat/reusable-module` (the `windows-cgo` job is the
CI embodiment of that branch's vendoring proof), updating PR #1.
