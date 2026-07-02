//go:build windows && cgo

/*
 * cgo_lz4.c — compiles the LZ4 reference implementation into the package.
 *
 * OpenVPN3's compress/lz4.hpp calls LZ4_compress_default / LZ4_decompress_safe
 * from the LZ4 library (HAVE_LZ4). We only have the header search path
 * (-I deps/lz4/lib); the implementation lives in deps/lz4/lib/lz4.c which cgo
 * cannot reach directly (it's in a subdir). Including it from this in-package C
 * TU emits the LZ4 symbols so the link resolves without an external -llz4.
 *
 * lz4.c is pinned at lz4-1.8.3 (see pkg/openvpn3/openvpn/deps/lib-versions and
 * scripts/openvpn3/fetch-deps.ps1).
 */

#include "deps/lz4/lib/lz4.c"
