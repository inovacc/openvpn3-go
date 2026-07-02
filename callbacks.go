//go:build windows && cgo

package openvpn3

/*
#include "ovpncli_shim.hpp"

// cgo cannot take the address of an //export'd Go function directly in Go code,
// so we assign the trampolines to the C callback-table fn pointers here, inside
// a C constructor. The exported Go symbols (lensrOnEvent, ...) are visible to C
// via the cgo-generated _cgo_export.h, which cgo #includes into this preamble
// automatically — so we do NOT redeclare them (cgo generates them with plain
// `char*`, not `const char*`; redeclaring with const triggers conflicting-type
// errors). The callback-table fn pointers expect `const char*`, so we cast each
// trampoline to the table's exact pointer type when assigning (a char* function
// is call-compatible; the cast just silences the const-qualifier mismatch).
// lensr_make_cb stamps user with the caller-supplied handle.
//
// Forward declarations MATCHING cgo's generated _cgo_export.h exactly (plain
// `char*`, no const) so this preamble compiles before that header is appended.
extern void lensrOnEvent(void* user, char* name, char* info, int err, int fatal);
extern void lensrOnLog(void* user, char* text);
extern int  lensrTunSetRemote(void* user, char* addr, int ipv6);
extern int  lensrTunAddAddress(void* user, char* addr, int prefix, char* gateway, int ipv6);
extern int  lensrTunAddRoute(void* user, char* addr, int prefix, int metric, int ipv6);
extern int  lensrTunAddDNS(void* user, char* ip);
extern int  lensrTunSetMTU(void* user, int mtu);
extern int  lensrTunEstablish(void* user);
extern void lensrTunTeardown(void* user);

static lensr_ovpn3_callbacks lensr_make_cb(void* user) {
  lensr_ovpn3_callbacks cb;
  cb.user           = user;
  cb.on_event       = (void (*)(void*, const char*, const char*, int, int))lensrOnEvent;
  cb.on_log         = (void (*)(void*, const char*))lensrOnLog;
  cb.tun_set_remote = (int (*)(void*, const char*, int))lensrTunSetRemote;
  cb.tun_add_address= (int (*)(void*, const char*, int, const char*, int))lensrTunAddAddress;
  cb.tun_add_route  = (int (*)(void*, const char*, int, int, int))lensrTunAddRoute;
  cb.tun_add_dns    = (int (*)(void*, const char*))lensrTunAddDNS;
  cb.tun_set_mtu    = lensrTunSetMTU;
  cb.tun_establish  = lensrTunEstablish;
  cb.tun_teardown   = lensrTunTeardown;
  return cb;
}
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

// newCClient builds the C callback table (wiring the //export'd trampolines via
// the static lensr_make_cb constructor in this file's preamble) and constructs
// the C client. It lives here — not in client.go — because a `static` C
// function in a cgo preamble is only visible within the SAME Go file's
// preamble; client.go cannot reference C.lensr_make_cb. Returns nil on alloc
// failure (caller frees the handle).
func newCClient(handle cgo.Handle) *C.lensr_ovpn3 {
	cb := C.lensr_make_cb(unsafe.Pointer(handle)) //nolint:govet // cgo.Handle passed as opaque C user-pointer (standard cgo idiom)
	return C.lensr_ovpn3_new(&cb)
}

// clientFromUser resolves the *Client behind the opaque C user pointer (a
// cgo.Handle). Returns nil if the handle is dead, in which case callbacks
// degrade gracefully (event/log dropped; tun verbs report failure).
func clientFromUser(user unsafe.Pointer) *Client {
	if user == nil {
		return nil
	}
	v := cgo.Handle(user).Value()
	cl, _ := v.(*Client)
	return cl
}

//export lensrOnEvent
func lensrOnEvent(user unsafe.Pointer, name, info *C.char, err, fatal C.int) {
	cl := clientFromUser(user)
	if cl == nil || cl.sink == nil {
		return
	}
	cl.sink.OnEvent(Event{
		Name:  C.GoString(name),
		Info:  C.GoString(info),
		Error: err != 0,
		Fatal: fatal != 0,
	})
}

//export lensrOnLog
func lensrOnLog(user unsafe.Pointer, text *C.char) {
	cl := clientFromUser(user)
	if cl == nil || cl.sink == nil {
		return
	}
	cl.sink.OnLog(C.GoString(text))
}

//export lensrTunSetRemote
func lensrTunSetRemote(user unsafe.Pointer, addr *C.char, ipv6 C.int) C.int {
	cl := clientFromUser(user)
	if cl == nil {
		return 1
	}
	cl.tun.spec.SetRemoteAddress(C.GoString(addr), ipv6 != 0)
	return 0
}

//export lensrTunAddAddress
func lensrTunAddAddress(user unsafe.Pointer, addr *C.char, prefix C.int, gateway *C.char, ipv6 C.int) C.int {
	cl := clientFromUser(user)
	if cl == nil {
		return 1
	}
	cl.tun.spec.AddAddress(C.GoString(addr), int(prefix), C.GoString(gateway), ipv6 != 0)
	return 0
}

//export lensrTunAddRoute
func lensrTunAddRoute(user unsafe.Pointer, addr *C.char, prefix, metric, ipv6 C.int) C.int {
	cl := clientFromUser(user)
	if cl == nil {
		return 1
	}
	cl.tun.spec.AddRoute(C.GoString(addr), int(prefix), int(metric), ipv6 != 0)
	return 0
}

//export lensrTunAddDNS
func lensrTunAddDNS(user unsafe.Pointer, ip *C.char) C.int {
	cl := clientFromUser(user)
	if cl == nil {
		return 1
	}
	cl.tun.spec.AddDNSServer(C.GoString(ip))
	return 0
}

//export lensrTunSetMTU
func lensrTunSetMTU(user unsafe.Pointer, mtu C.int) C.int {
	cl := clientFromUser(user)
	if cl == nil {
		return 1
	}
	cl.tun.spec.SetMTU(int(mtu))
	return 0
}

//export lensrTunEstablish
func lensrTunEstablish(user unsafe.Pointer) C.int {
	cl := clientFromUser(user)
	if cl == nil {
		return -1
	}
	if err := cl.tun.Establish(); err != nil {
		cl.sink.OnLog("openvpn3: tun establish failed: " + err.Error())
		return -1
	}
	// The flat ABI expects a tun fd/handle (>=0). The actual TUN device is owned
	// by the elevated helper out-of-process, so there is no in-process fd to
	// hand back; return 0 to signal "established" without exposing a handle.
	return 0
}

//export lensrTunTeardown
func lensrTunTeardown(user unsafe.Pointer) {
	cl := clientFromUser(user)
	if cl == nil {
		return
	}
	if err := cl.tun.Teardown(); err != nil {
		cl.sink.OnLog("openvpn3: tun teardown failed: " + err.Error())
	}
}
