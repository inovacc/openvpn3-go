/*
 * ovpncli_shim.hpp — flat C ABI over OpenVPN3 ClientAPI::OpenVPNClient.
 *
 * This is the stable contract the Go cgo layer (Task 8) is written against.
 * Keep these names/shapes EXACTLY; do not reorder struct fields or change
 * parameter lists without coordinating a matching change in the cgo binding.
 *
 * Pinned against OpenVPN3 submodule tag release/3.11.6
 * (commit 5b7841a847619e9e1ba3f7371e0c9e2743383481).
 */
#pragma once
#include <stddef.h>
#ifdef __cplusplus
extern "C" {
#endif

/*
 * Callback table the C++ shim invokes on the Go side. Every function pointer
 * receives the opaque `user` handle (a cgo.Handle on the Go side) as its first
 * argument. All const char* are NUL-terminated UTF-8 owned by the shim for the
 * duration of the call only (callee must copy if it needs to retain).
 */
typedef struct lensr_ovpn3_callbacks {
  void* user; /* opaque cgo.Handle */

  /* Connection lifecycle event. err/fatal are 0/1. */
  void (*on_event)(void* user, const char* name, const char* info, int err, int fatal);

  /* Log line. */
  void (*on_log)(void* user, const char* text);

  /* TunBuilder: remote VPN server address. ipv6 is 0/1. Returns 0 on success. */
  int (*tun_set_remote)(void* user, const char* addr, int ipv6);

  /* TunBuilder: add a local tunnel address. ipv6 is 0/1. gateway may be empty. */
  int (*tun_add_address)(void* user, const char* addr, int prefix, const char* gateway, int ipv6);

  /* TunBuilder: add a route. ipv6 is 0/1. Returns 0 on success. */
  int (*tun_add_route)(void* user, const char* addr, int prefix, int metric, int ipv6);

  /* TunBuilder: add a DNS server IP (one call per server). Returns 0 on success. */
  int (*tun_add_dns)(void* user, const char* ip);

  /* TunBuilder: set MTU. Returns 0 on success. */
  int (*tun_set_mtu)(void* user, int mtu);

  /* TunBuilder: bring the tunnel up. Returns the tun fd/handle (>=0) or -1. */
  int (*tun_establish)(void* user);

  /* TunBuilder: tear the tunnel down. */
  void (*tun_teardown)(void* user);
} lensr_ovpn3_callbacks;

/* Opaque handle wrapping the C++ LensrClient instance. */
typedef struct lensr_ovpn3 lensr_ovpn3;

/* Construct a client. Copies the callback table. Returns NULL on alloc failure. */
lensr_ovpn3* lensr_ovpn3_new(const lensr_ovpn3_callbacks* cb);

/*
 * Parse the .ovpn profile and (when user is non-empty) stage credentials.
 * Returns 1 on error and writes a NUL-terminated message into msg (bounded by
 * msg_len); returns 0 on success.
 */
int lensr_ovpn3_eval_config(lensr_ovpn3* c, const char* content,
                            const char* user, const char* pass,
                            char* msg, size_t msg_len);

/*
 * Connect. BLOCKS until the session ends (stop()/error). Returns 1 on error and
 * writes a NUL-terminated message into msg (bounded by msg_len); returns 0 on
 * clean disconnect.
 */
int lensr_ovpn3_connect(lensr_ovpn3* c, char* msg, size_t msg_len);

/* Request an async stop. Safe to call from another thread than connect(). */
void lensr_ovpn3_stop(lensr_ovpn3* c);

/* Snapshot transport byte counters. Returns 0 on success, 1 on error. */
int lensr_ovpn3_transport_stats(lensr_ovpn3* c, long long* in, long long* out);

/* Destroy the client and free all resources. */
void lensr_ovpn3_free(lensr_ovpn3* c);

#ifdef __cplusplus
}
#endif
