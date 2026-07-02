//go:build windows && cgo

/*
 * cgo_bridge.cpp — translation unit #1 of the OpenVPN3 cgo binding: the
 * OpenVPN3 core compiled as a single TU.
 *
 * IMPORTANT (two-TU split): the shim is compiled in a SEPARATE TU
 * (cgo_shim_windows.cpp), NOT here. The reason is that
 * openvpn/client/ovpncli.hpp has NO include guard / no `#pragma once` — it is
 * designed to be included exactly once per translation unit (via ovpncli.cpp).
 * If this file pulled in BOTH ovpncli.cpp AND the shim (which itself includes
 * "ovpncli.hpp"), the API structs would be defined twice in one TU and fail to
 * compile (redefinition of openvpn::ClientAPI::EvalConfig, ...). Splitting into
 * two TUs gives each a single inclusion of the header; cgo compiles every .cpp
 * in the package dir as its own object and links them together, so the shim's
 * use of the public API resolves against the impl symbols emitted here.
 *
 * cgo only builds C/C++ files that sit DIRECTLY in pkg/openvpn3/, which is why
 * the core (openvpn/client/ovpncli.cpp) is reached via #include here rather
 * than compiled directly. Include roots, crypto backend, asio, lz4, and the
 * Windows TAP header all come from the #cgo directives in cgo_flags.go.
 */

/* Match cli.cpp: keep core symbols hidden (also set via -D for safety). */
#ifndef OPENVPN_CORE_API_VISIBILITY_HIDDEN
#define OPENVPN_CORE_API_VISIBILITY_HIDDEN
#endif

/* Platform detection header (defines OPENVPN_PLATFORM_WIN etc). */
#include <openvpn/common/platform.hpp>

/*
 * OpenVPN3 core as a single translation unit. This is the heavy include: it
 * compiles the entire client stack (transport, crypto, tun, options parser)
 * and emits the implementation symbols the shim links against. It includes
 * "ovpncli.hpp" exactly once.
 */
#include <client/ovpncli.cpp>
