# openvpn3-go
<!-- rev:003 -->

[![test](https://github.com/inovacc/openvpn3-go/actions/workflows/test.yml/badge.svg)](https://github.com/inovacc/openvpn3-go/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/inovacc/openvpn3-go.svg)](https://pkg.go.dev/github.com/inovacc/openvpn3-go)
[![Go Version](https://img.shields.io/github/go-mod/go-version/inovacc/openvpn3-go)](go.mod)
[![License: BSD 3-Clause](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](LICENSE)

> A daemon application built on [mantle](https://github.com/inovacc/mantle).

## Using the module

```bash
go get github.com/inovacc/openvpn3-go
```

Pure-Go (stub engine, no C toolchain): build with `CGO_ENABLED=0`. The API is
identical; `openvpn3.Available()` reports `false` and connects return
`ErrUnavailable`.

Real engine (Windows): build with `CGO_ENABLED=1` and a MinGW-w64 gcc on PATH.
All C/C++ sources (OpenVPN3 core, asio, lz4, jsoncpp, tap-windows.h) are
vendored in the module — no bootstrap step, no `replace` directive, no writable
checkout needed. Only OpenSSL comes from your toolchain (next section).

## Linking OpenSSL

The cgo engine links `-lssl -lcrypto`. If your gcc ships OpenSSL on its
default search paths (Linux distros, msys2 ucrt64), it just works. If you see:

```
fatal error: openssl/opensslv.h: No such file or directory
```

or at link time:

```
cannot find -lssl / cannot find -lcrypto
```

point cgo at an OpenSSL dev install once:

```bash
export CGO_CPPFLAGS="-I/path/to/ssl/include"
export CGO_LDFLAGS="-L/path/to/ssl/lib"
```

Developers working in a source checkout can instead run
`go run ./cmd/openvpn bootstrap`, which detects OpenSSL and writes a
gitignored `cgo_openssl_local.go` so no env is needed.

## Build

```bash
task build      # or: go build ./...
```

## Test

```bash
task test       # fast tests
task test:full  # full suite
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright (c) 2026 dyammarcano.
