//go:build windows && cgo

/*
 * cgo_shim_windows.cpp — translation unit #2 of the OpenVPN3 cgo binding: the
 * flat C ABI shim (LensrClient : openvpn::ClientAPI::OpenVPNClient).
 *
 * Compiled SEPARATELY from cgo_bridge.cpp (the core TU) — see the long comment
 * in cgo_bridge.cpp for why: openvpn/client/ovpncli.hpp has no include guard,
 * so the core and the shim must each include it in their own TU. This TU
 * includes the shim's .cpp, which includes "ovpncli.hpp" once (the PUBLIC
 * interface only). The OpenVPN3 implementation symbols the shim calls
 * (eval_config, connect, provide_creds, transport_stats, the OpenVPNClient
 * ctor/dtor + vtable) are emitted by cgo_bridge.cpp and resolved at link.
 *
 * Visibility/platform defines mirror cgo_bridge.cpp so both TUs agree on the
 * ABI of the shared openvpn:: types.
 */

#ifndef OPENVPN_CORE_API_VISIBILITY_HIDDEN
#define OPENVPN_CORE_API_VISIBILITY_HIDDEN
#endif

#include <openvpn/common/platform.hpp>

/*
 * The shim implementation. It includes ovpncli_shim.hpp (the C ABI) and
 * "ovpncli.hpp" (the public interface, resolved via -I openvpn/client), then
 * defines the extern "C" entry points the cgo layer calls.
 */
#include "ovpncli_shim.cpp"
