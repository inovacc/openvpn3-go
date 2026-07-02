//go:build windows && cgo

package openvpn3

// cgo_flags.go centralizes every #cgo directive for the OpenVPN3 binding so the
// other tagged Go files stay free of build plumbing. cgo compiles only C/C++
// translation units that live DIRECTLY in this package directory
// (pkg/openvpn3/), never in subdirectories — so cgo_bridge.cpp (a sibling of
// this file) is the single TU that #includes both the shim impl (shim/) and the
// OpenVPN3 single-TU core (openvpn/client/ovpncli.cpp).
//
// Include path roots:
//   -I${SRCDIR}/openvpn        => <client/ovpncli.hpp>, <openvpn/...>
//   -I${SRCDIR}/shim           => "ovpncli_shim.hpp"
//   -I${SRCDIR}/deps/asio/...  => <asio.hpp>            (header-only, USE_ASIO)
//   -I${SRCDIR}/deps/lz4/lib   => <lz4.h>               (HAVE_LZ4)
//   -I${SRCDIR}/deps/jsoncpp/include => <json/json.h>
//
// Required preprocessor defines (mirrors OpenVPN3 CMake + io/io.hpp):
//   USE_OPENSSL            select the OpenSSL crypto/SSL backend
//   USE_ASIO + ASIO_STANDALONE  io/io.hpp gates asio.hpp behind USE_ASIO;
//                          ASIO_STANDALONE keeps asio header-only (no Boost)
//   HAVE_LZ4               enable LZ4 compression support
//   OPENVPN_CORE_API_VISIBILITY_HIDDEN  match cli.cpp (don't export core syms)
//   _WIN32_WINNT=0x0601    Windows 7+ socket/API surface asio needs
//   WIN32_LEAN_AND_MEAN / NOMINMAX  tame <windows.h> for C++ headers
//
// Link libraries:
//   -lssl -lcrypto         OpenSSL. NOTE: the MinGW *gcc* package on this machine
//                          is a SEPARATE scoop install from the *mingw* package
//                          that ships the OpenSSL dev headers/libs (opt/include +
//                          opt/lib/libssl.a). They are NOT on gcc's default search
//                          path, so the build REQUIRES pointing cgo at them via
//                          env at build time (these are machine-specific and must
//                          not be hardcoded in committed source):
//                            $ssl = "<mingw-pkg>\opt"
//                            CGO_CPPFLAGS=-I$ssl\include  CGO_LDFLAGS=-L$ssl\lib
//                          If your toolchain bundles OpenSSL on the default path
//                          (e.g. a unified MSYS2 mingw64), no env is needed.
//   Windows system libs    ws2_32 (sockets), iphlpapi (interface/route info),
//                          crypt32 (cert store), fwpuclnt (WFP), ole32/oleaut32,
//                          advapi32, gdi32, user32, bcrypt (RNG), wininet,
//                          setupapi, wtsapi32, rpcrt4 (UuidCreate), uuid
//                          (GUID_DEVCLASS_NET et al), version
//   -lstdc++               C++ runtime (cgo links with gcc, not g++)
//
// LZ4 + the OpenVPN3 epoch crypto impl are compiled IN-PACKAGE as extra TUs
// (cgo_lz4.c includes deps/lz4/lib/lz4.c; cgo_core_epoch_windows.cpp includes
// openvpn/crypto/data_epoch.cpp) because ovpncli.cpp's single-TU does not emit
// those symbols. No external -llz4 is needed.

/*
#cgo CPPFLAGS: -I${SRCDIR}/openvpn -I${SRCDIR}/openvpn/client -I${SRCDIR}/shim -I${SRCDIR}/deps/asio/asio/include -I${SRCDIR}/deps/lz4/lib -I${SRCDIR}/deps/jsoncpp/include -I${SRCDIR}/deps/tap-windows/include
#cgo CPPFLAGS: -DUSE_OPENSSL -DUSE_ASIO -DASIO_STANDALONE -DHAVE_LZ4 -DOPENVPN_CORE_API_VISIBILITY_HIDDEN
#cgo CPPFLAGS: -D_WIN32_WINNT=0x0601 -DWIN32_LEAN_AND_MEAN -DNOMINMAX
// TAP_WIN_COMPONENT_ID is "generally defined on cl command line" by the upstream
// OpenVPN3 build (tunutil.hpp); without it COMPONENT_ID stringizes to the literal
// token "TAP_WIN_COMPONENT_ID" and matches NO adapter, so the TapWindows6 path
// finds zero TAP devices ("cannot acquire TAP handle"). lensr's TAP driver comes
// from OpenVPN Connect, whose adapter ComponentId is `tap_ovpnconnect` (the same
// adapter the native connector uses) — point the OpenVPN3 enumeration at it.
#cgo CPPFLAGS: -DTAP_WIN_COMPONENT_ID=tap_ovpnconnect
#cgo CXXFLAGS: -std=c++17 -fexceptions -Wno-deprecated-declarations
#cgo LDFLAGS: -lssl -lcrypto -lstdc++
#cgo LDFLAGS: -lws2_32 -liphlpapi -lcrypt32 -lfwpuclnt -lole32 -loleaut32 -ladvapi32 -lgdi32 -luser32 -lbcrypt -lwininet -lsetupapi -lwtsapi32 -lrpcrt4 -luuid -lversion
*/
import "C"
