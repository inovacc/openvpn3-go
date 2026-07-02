/*
 * ovpncli_shim.cpp — implementation of the flat C ABI declared in
 * ovpncli_shim.hpp. Subclasses openvpn::ClientAPI::OpenVPNClient and forwards
 * the event/log/TunBuilder callbacks to a C callback table so a Go cgo layer
 * (Task 8) can drive the VPN without touching C++ types.
 *
 * Verified against OpenVPN3 submodule tag release/3.11.6
 * (commit 5b7841a847619e9e1ba3f7371e0c9e2743383481). Every override below cites
 * the header:line of the real signature it matches.
 *
 * Build note for Task 8: the official example (test/ovpncli/cli.cpp:53) compiles
 * the API as a single translation unit via `#include <client/ovpncli.cpp>`.
 * This shim includes only the header; the cgo build MUST either compile
 * openvpn/client/ovpncli.cpp alongside this file, or switch the include below to
 * the .cpp. Include path root: pkg/openvpn3/openvpn (so <client/ovpncli.hpp> and
 * <openvpn/...> resolve). See report for the full library/define list.
 */
#include "ovpncli_shim.hpp"

#include <cstring>
#include <new>
#include <string>

#include "ovpncli.hpp" // openvpn::ClientAPI::OpenVPNClient + structs (client/ovpncli.hpp)

namespace {

// Copy a std::string into a caller-provided bounded buffer, always NUL-terminating.
void copy_msg(char *dst, size_t dst_len, const std::string &src) {
  if (dst == nullptr || dst_len == 0) {
    return;
  }
  const size_t n = src.size() < (dst_len - 1) ? src.size() : (dst_len - 1);
  std::memcpy(dst, src.data(), n);
  dst[n] = '\0';
}

// LensrClient subclasses the real OpenVPN3 client and forwards every callback
// it overrides to the C callback table captured at construction.
class LensrClient : public openvpn::ClientAPI::OpenVPNClient {
public:
  explicit LensrClient(const lensr_ovpn3_callbacks &cb) : cb_(cb) {}

  // ---- Pure-virtual ClientAPI callbacks (MUST be implemented) ----

  // ovpncli.hpp:722 — virtual void event(const Event &) = 0;
  void event(const openvpn::ClientAPI::Event &ev) override {
    if (cb_.on_event) {
      cb_.on_event(cb_.user, ev.name.c_str(), ev.info.c_str(),
                   ev.error ? 1 : 0, ev.fatal ? 1 : 0);
    }
  }

  // ovpncli.hpp:725 — virtual void acc_event(const AppCustomControlMessageEvent &) = 0;
  // App custom control channel events are out of scope for v0; ignore.
  void acc_event(const openvpn::ClientAPI::AppCustomControlMessageEvent & /*acev*/) override {
  }

  // ovpncli.hpp:729 — virtual void log(const LogInfo &) override = 0;
  // LogInfo.text — ovpncli.hpp:446.
  void log(const openvpn::ClientAPI::LogInfo &li) override {
    if (cb_.on_log) {
      cb_.on_log(cb_.user, li.text.c_str());
    }
  }

  // ovpncli.hpp:664 — virtual bool pause_on_connection_timeout() = 0;
  // false => core disconnects on connection timeout (no PAUSE state). v0 wants a
  // hard disconnect so the Go side observes the failure event.
  bool pause_on_connection_timeout() override { return false; }

  // ovpncli.hpp:733 — virtual void external_pki_cert_request(ExternalPKICertRequest &) = 0;
  // External PKI is unsupported in v0. Mirror test/ovpncli/cli.cpp:505-520:
  // signal not-implemented via the request's error fields (ExternalPKIRequestBase,
  // ovpncli.hpp:491-497: bool error (493); std::string errorText (494)).
  void external_pki_cert_request(openvpn::ClientAPI::ExternalPKICertRequest &certreq) override {
    certreq.error = true;
    certreq.errorText = "external_pki_cert_request not implemented";
  }

  // ovpncli.hpp:734 — virtual void external_pki_sign_request(ExternalPKISignRequest &) = 0;
  // Same not-implemented handling as cli.cpp:682-750.
  void external_pki_sign_request(openvpn::ClientAPI::ExternalPKISignRequest &signreq) override {
    signreq.error = true;
    signreq.errorText = "external_pki_sign_request not implemented";
  }

  // ---- TunBuilderBase overrides (base.hpp) ----

  // base.hpp:83 — virtual bool tun_builder_set_remote_address(const std::string &address, bool ipv6);
  bool tun_builder_set_remote_address(const std::string &address, bool ipv6) override {
    if (!cb_.tun_set_remote) {
      return false;
    }
    return cb_.tun_set_remote(cb_.user, address.c_str(), ipv6 ? 1 : 0) == 0;
  }

  // base.hpp:101-105 — virtual bool tun_builder_add_address(const std::string &address,
  //                       int prefix_length, const std::string &gateway, bool ipv6, bool net30);
  // net30 is not forwarded across the C ABI (v0 does not need it).
  bool tun_builder_add_address(const std::string &address, int prefix_length,
                               const std::string &gateway, bool ipv6,
                               bool /*net30*/) override {
    if (!cb_.tun_add_address) {
      return false;
    }
    return cb_.tun_add_address(cb_.user, address.c_str(), prefix_length,
                               gateway.c_str(), ipv6 ? 1 : 0) == 0;
  }

  // base.hpp:158-161 — virtual bool tun_builder_add_route(const std::string &address,
  //                       int prefix_length, int metric, bool ipv6);
  bool tun_builder_add_route(const std::string &address, int prefix_length,
                             int metric, bool ipv6) override {
    if (!cb_.tun_add_route) {
      return false;
    }
    return cb_.tun_add_route(cb_.user, address.c_str(), prefix_length, metric,
                             ipv6 ? 1 : 0) == 0;
  }

  // base.hpp:195 — virtual bool tun_builder_set_dns_options(const DnsOptions &dns);
  // 3.11.6 has NO tun_builder_add_dns_server; DNS arrives as a single DnsOptions
  // struct (openvpn/client/dns_options.hpp). Field path to each server address:
  //   DnsOptions.servers  (std::map<int, DnsServer>, dns_options.hpp:623)
  //     -> DnsServer.addresses (std::vector<DnsAddress>, dns_options.hpp:505)
  //       -> DnsAddress.address (std::string, dns_options.hpp:155)
  // We iterate priority-ordered (std::map keeps int keys sorted) and emit one
  // tun_add_dns callback per address. Search domains (DnsOptions.search_domains,
  // dns_options.hpp:622, and per-server DnsServer.domains, dns_options.hpp:508)
  // are deferred — out of scope for v0.
  bool tun_builder_set_dns_options(const openvpn::DnsOptions &dns) override {
    if (!cb_.tun_add_dns) {
      return false;
    }
    bool ok = true;
    for (const auto &entry : dns.servers) {
      const openvpn::DnsServer &server = entry.second;
      for (const auto &addr : server.addresses) {
        if (cb_.tun_add_dns(cb_.user, addr.address.c_str()) != 0) {
          ok = false;
        }
      }
    }
    return ok;
  }

  // base.hpp:211 — virtual bool tun_builder_set_mtu(int mtu);
  bool tun_builder_set_mtu(int mtu) override {
    if (!cb_.tun_set_mtu) {
      return false;
    }
    return cb_.tun_set_mtu(cb_.user, mtu) == 0;
  }

  // base.hpp:361 — virtual int tun_builder_establish(); returns the tun fd, -1 on failure.
  int tun_builder_establish() override {
    if (!cb_.tun_establish) {
      return -1;
    }
    return cb_.tun_establish(cb_.user);
  }

  // base.hpp:417 — virtual void tun_builder_teardown(bool disconnect);
  void tun_builder_teardown(bool /*disconnect*/) override {
    if (cb_.tun_teardown) {
      cb_.tun_teardown(cb_.user);
    }
  }

private:
  lensr_ovpn3_callbacks cb_;
};

} // namespace

// Concrete definition of the opaque handle: just owns a LensrClient.
struct lensr_ovpn3 {
  LensrClient client;
  explicit lensr_ovpn3(const lensr_ovpn3_callbacks &cb) : client(cb) {}
};

extern "C" {

lensr_ovpn3 *lensr_ovpn3_new(const lensr_ovpn3_callbacks *cb) {
  if (cb == nullptr) {
    return nullptr;
  }
  return new (std::nothrow) lensr_ovpn3(*cb);
}

int lensr_ovpn3_eval_config(lensr_ovpn3 *c, const char *content,
                            const char *user, const char *pass,
                            char *msg, size_t msg_len) {
  if (c == nullptr) {
    copy_msg(msg, msg_len, "null client");
    return 1;
  }

  // ovpncli.hpp:617 — EvalConfig eval_config(const Config &);
  // Config.content — ovpncli.hpp:350.
  openvpn::ClientAPI::Config config;
  config.content = content ? content : "";

  // Tun backend selection (ovpncli.hpp:321 Config::wintun). cliopt.hpp:490 maps
  // this bool: wintun ? TunWin::Wintun : TunWin::TapWindows6. We use the legacy
  // TAP-Windows6 path (wintun=false) deliberately:
  //
  //   * tun_type==TapWindows6 makes tunutil.hpp::tap_guids() enumerate adapters
  //     whose registry ComponentId equals COMPONENT_ID, which cgo_flags.go defines
  //     to `tap_ovpnconnect` — the ComponentId of the TAP-Windows Adapter V9 that
  //     OpenVPN Connect installs. That is the SAME adapter the native connector
  //     opens successfully on this host, so the device is known to be acquirable.
  //
  //   * wintun=true was tried first and FAILS here: TunWin::Wintun makes tap_guids
  //     search for ComponentId "wintun", but no Wintun adapter is installed (the
  //     host has only tap_ovpnconnect + ovpn-dco), so enumeration returns empty and
  //     the connect dies at TUN_IFACE_CREATE ("cannot acquire TAP handle") even
  //     though the SSL handshake + PUSH_REPLY completed.
  //
  // REQUIRES the -DTAP_WIN_COMPONENT_ID=tap_ovpnconnect define in cgo_flags.go;
  // without it COMPONENT_ID stringizes to a literal that matches no adapter.
  config.wintun = false;

  // EvalConfig.error / .message — ovpncli.hpp:57/60.
  openvpn::ClientAPI::EvalConfig eval = c->client.eval_config(config);
  if (eval.error) {
    copy_msg(msg, msg_len, eval.message);
    return 1;
  }

  // Stage credentials only when a username is supplied (autologin profiles omit it).
  if (user != nullptr && user[0] != '\0') {
    // ovpncli.hpp:620 — Status provide_creds(const ProvideCreds &);
    // ProvideCreds.username/.password — ovpncli.hpp:115/116.
    openvpn::ClientAPI::ProvideCreds creds;
    creds.username = user;
    creds.password = pass ? pass : "";

    // Status.error / .message — ovpncli.hpp:430/432.
    openvpn::ClientAPI::Status st = c->client.provide_creds(creds);
    if (st.error) {
      copy_msg(msg, msg_len, st.message);
      return 1;
    }
  }

  copy_msg(msg, msg_len, "");
  return 0;
}

int lensr_ovpn3_connect(lensr_ovpn3 *c, char *msg, size_t msg_len) {
  if (c == nullptr) {
    copy_msg(msg, msg_len, "null client");
    return 1;
  }

  // ovpncli.hpp:631 — Status connect(); BLOCKS until disconnect.
  // Status.error / .message — ovpncli.hpp:430/432.
  openvpn::ClientAPI::Status st = c->client.connect();
  if (st.error) {
    copy_msg(msg, msg_len, st.message);
    return 1;
  }

  copy_msg(msg, msg_len, "");
  return 0;
}

void lensr_ovpn3_stop(lensr_ovpn3 *c) {
  if (c != nullptr) {
    // ovpncli.hpp:645 — void stop(); async-safe from another thread.
    c->client.stop();
  }
}

int lensr_ovpn3_transport_stats(lensr_ovpn3 *c, long long *in, long long *out) {
  if (c == nullptr) {
    return 1;
  }
  // ovpncli.hpp:685 — TransportStats transport_stats() const;
  // TransportStats.bytesIn/.bytesOut (long long) — ovpncli.hpp:470/471.
  openvpn::ClientAPI::TransportStats ts = c->client.transport_stats();
  if (in != nullptr) {
    *in = ts.bytesIn;
  }
  if (out != nullptr) {
    *out = ts.bytesOut;
  }
  return 0;
}

void lensr_ovpn3_free(lensr_ovpn3 *c) {
  delete c; // ~LensrClient -> ~OpenVPNClient
}

} // extern "C"
