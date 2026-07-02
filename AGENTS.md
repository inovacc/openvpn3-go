# AGENTS.md
<!-- rev:003 -->

## Project overview

`github.com/inovacc/openvpn3-go` — a standalone **cgo** Go module wrapping the OpenVPN3 core. Exposes a public `Tunnel` interface (`tunnel.go`, `client.go`, `profile.go`, `types.go`) with per-OS engine implementations (`engine_windows.go`, `engine_darwin.go`, `engine_unix.go`, `engine_stub.go`) and cgo bridges (`cgo_bridge.cpp`, `cgo_shim_windows.cpp`, `callbacks.go`). C/C++ sources (OpenVPN3 core pinned at `5b7841a`, asio, lz4, jsoncpp, tap-windows.h) are **vendored** under `openvpn/` and `deps/`; `tools/` (nested module) holds the maintainer `vendorsync`/`modverify` tools. CLI lives at `cmd/openvpn`. Go 1.25.0; sole dependency `golang.org/x/sys`.

## Dev environment tips

- This is a **cgo** module — a C/C++ toolchain is required to build. No bootstrap needed — C/C++ deps are vendored. On Windows, run `go run ./cmd/openvpn bootstrap` once if OpenSSL isn't on your toolchain's default paths (writes gitignored `cgo_openssl_local.go`).
- Prefer the **Taskfile** for everything (`task --list`). Binary = `openvpn`, main = `./cmd/openvpn`.
- Pure-Go paths build without the core via the stub engine (`engine_stub.go`, `stub.go`).

## Build & test commands

Prefer task-runner names (a `Taskfile.yml` is present):

- Build: `task build`  (native: `go build ./...`)
- Run CLI: `task run`
- Install: `task install`
- Format / vet / lint: `task fmt` · `task vet` · `task lint` (autofix: `task lint:fix` or `task fix`)
- Test (fast, `-short`): `task test`
- Test (full suite, incl. slow/integration): `task test:full`
- Coverage: `task test:coverage` (or `task test:cover`)
- Combined gate: `task check`
- Deps: `task deps` · `task deps:upgrade`
- Release: `task release` · `task release:snapshot` · `task release:check`

The suite must pass before merge. Slow / live tests (`live_integration_test.go`) are gated under `testing.Short()` — run them with `task test:full`.

## Code style

- Idiomatic Go. `gofmt`/`go vet` clean; `golangci-lint` clean (`task lint`).
- Wrap errors with `%w`; compare with `errors.Is` / `errors.As`, never `==`.
- Platform splits use the `_windows.go` / `_darwin.go` / `_unix.go` / `_stub.go` suffix convention already in the tree — match it for new OS-specific code.
- Keep cgo (`import "C"`) confined to the existing bridge files; do not scatter cgo across packages.

## Testing instructions

- Add or update table-driven tests alongside changed code (`*_test.go`); existing examples: `profile_test.go`, `tunspec_test.go`, `types_test.go`, `stub_test.go`.
- Gate anything needing the real core / network / >5s under `testing.Short()` so `task test` stays fast.

## Security

- Never commit secrets, VPN credentials, or `.ovpn` profiles with embedded keys.
- Validate all profile / config input before passing into cgo.
- Verify the vendored C/C++ tree checksums with `task vendor:verify` against the provenance table in `NOTICE.md` (see also `SIGNATURES.md`).

## PR / commit instructions

- Conventional commits (`feat:`, `fix:`, `docs:`, …) — matches existing history.
- Run `task check` (fmt + vet + lint + test) before committing.
- No AI attribution in commit messages.
