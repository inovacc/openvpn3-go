# Design: openvpn3-go as a plain `go get`-able module

**Date:** 2026-07-02
**Status:** Approved

## Problem

The cgo engine's `#cgo` directives use `${SRCDIR}`-relative include paths
(`${SRCDIR}/openvpn`, `${SRCDIR}/deps/...`). Those trees exist only after
`openvpn bootstrap` writes them into the module root, which requires a
**writable** checkout — impossible in the read-only Go module cache. Consumers
therefore need a local clone + `replace` directive. The stub path
(`CGO_ENABLED=0`) is already `go get`-able; only the real engine is not.

## Goal

`go get github.com/inovacc/openvpn3-go` + a C toolchain (gcc/mingw) =
working cgo engine. No `replace`, no writable module root, no bootstrap step
for consumers. `CGO_ENABLED=0` stub builds keep working untouched.

## Decisions

1. **Vendor the pinned C/C++ sources in-repo** (approx. 12.6 MB, ~1,250 files),
   at their **current paths** so `cgo_flags.go` needs zero changes.
2. **OpenSSL stays external**, wired by consumer env vars — no OpenSSL paths in
   committed `#cgo` flags.
3. Chosen over the alternatives considered: prebuilt runtime-loaded DLLs
   (cleanest UX but a much larger engineering and distribution change) and a
   shared bootstrap cache dir (no repo bloat but still not plain `go get`).

## 1. Vendored trees

Commit pruned copies of what the build actually includes:

| Path | Content | Origin pin |
|------|---------|-----------|
| `openvpn/openvpn/`, `openvpn/client/` | OpenVPN3 core headers + single-TU client | SHA `5b7841a847619e9e1ba3f7371e0c9e2743383481` |
| `deps/asio/asio/include/` | asio headers only | tag `asio-1-24-0` |
| `deps/lz4/lib/` | lz4 sources | tag `v1.8.3` |
| `deps/jsoncpp/include/` (+ `src/` only if the single-TU build needs it — verify at implementation) | jsoncpp | tag `1.8.4` |
| `deps/tap-windows/include/` | `tap-windows.h` | tap-windows6 |

- The `call.hpp` CREATE_NO_WINDOW patch is applied **at vendor time**; the
  committed copy is the patched copy.
- Each vendored tree carries its upstream LICENSE file.
- `.git` dirs, docs, tests, and examples are dropped.
- Remove the corresponding `/openvpn` and `/deps` ignore entries from
  `.gitignore`.
- Add a module-zip sanity check (case-insensitive filename collisions, size)
  run before tagging a release.

## 2. OpenSSL linkage

- Default committed flags contain no OpenSSL paths. Linux and msys2-ucrt64
  toolchains find OpenSSL on default search paths; other setups set
  `CGO_CPPFLAGS=-I<ssl>/include CGO_LDFLAGS=-L<ssl>/lib` once.
- The generated, gitignored `cgo_openssl_local.go` **remains** as a
  maintainer/dev convenience — untracked files never ship in module zips.
- README gains a "linking OpenSSL" section keyed to the exact compile/link
  error a consumer would see without it.

## 3. Bootstrap becomes a maintainer tool

- `bootstrap/` public package is **deleted** (v0.1.0, no external consumers —
  acceptable breakage; the standing deprecation policy is waived here by
  explicit decision).
- Its fetch/prune/patch logic moves to `internal/vendorsync` (maintainer-only):
  re-fetch pinned SHAs → prune → apply patch → verify checksums against
  `SIGNATURES.md` → refresh the vendored trees.
- The `openvpn bootstrap` CLI subcommand shrinks to the dev-machine helper
  that detects OpenSSL and writes `cgo_openssl_local.go`.

## 4. Licensing / provenance

Vendoring OpenVPN3 puts AGPLv3 code in the repo. `NOTICE.md` and
`SIGNATURES.md` list every vendored tree's origin URL, pinned SHA/tag,
license, and checksum. (Consumers already linked AGPL code when compiling via
bootstrap; vendoring changes redistribution, not the linking story.)

## 5. Verification

1. `CGO_ENABLED=0 go build ./...` and `go test -short ./...` stay green.
2. `CGO_ENABLED=1 go build ./...` succeeds from a **fresh clone with no
   bootstrap step** (OpenSSL env set as documented).
3. Proof of reusability: a scratch consumer project depends on the module via
   a module-cache path (`go mod download` zip or pseudo-version) and builds the
   cgo engine read-only.
4. `docs/ROADMAP.md` and `docs/INTEGRATION.md` reconciled to this model.
