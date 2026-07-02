# OpenVPN3 ClientAPI / TunBuilderBase — REAL signatures (source of truth)

Pinned vendored tag: **`release/3.11.6`**
Vendored commit: **`5b7841a847619e9e1ba3f7371e0c9e2743383481`**
Captured: 2026-06-02

This file records the EXACT C++ signatures/fields for the pinned tag. The C++ shim
(T7) and the cgo binding (T8) MUST match these byte-for-byte. Anything that differs
from the original task assumptions is flagged with **[DIFF]**.

All line refs are relative to the vendored tree root `openvpn/`.

---

## 1. `ClientAPI::OpenVPNClient` methods — `client/ovpncli.hpp`

The class is declared `class OpenVPNClient : public TunBuilderBase, ... private ExternalPKIBase`
(line 609-611). Subclass it to override the pure-virtual callbacks (`event`, `log`,
`pause_on_connection_timeout`) and the TunBuilderBase callbacks.

```cpp
// ovpncli.hpp:613-614
OpenVPNClient();
virtual ~OpenVPNClient();

// ovpncli.hpp:617  — Parse OpenVPN configuration file.
EvalConfig eval_config(const Config &);

// ovpncli.hpp:620  — Provide credentials. Call before connect().
Status provide_creds(const ProvideCreds &);

// ovpncli.hpp:631  — Primary connect; BLOCKS until disconnect. Returns Status.
Status connect();                       // [DIFF] returns Status, NOT void

// ovpncli.hpp:645  — Stop the client. Async-safe from another thread.
void stop();

// ovpncli.hpp:685  — Transport stats snapshot (const).
TransportStats transport_stats() const;
```

Pure-virtual callbacks the subclass MUST implement:

```cpp
// ovpncli.hpp:722  — event delivery during connect().
virtual void event(const Event &) = 0;

// ovpncli.hpp:729  — log delivery during connect().
virtual void log(const LogInfo &) override = 0;     // NOTE: declared `override` (overrides LogReceiver::log)

// ovpncli.hpp:664  — must be implemented; return false => disconnect on timeout.
virtual bool pause_on_connection_timeout() = 0;
```

> **[DIFF / NOTE]** there is a SECOND `eval_config` at `ovpncli.hpp:573`
> (`EvalConfig eval_config(const Config &config);`) inside a different/nested
> declaration block; the public one to call is **line 617**. The shim should
> call the public member on the subclass instance.

Other useful (not required) members: `connection_info()` (635), `session_token()` (640),
`pause()` (650), `resume()` (654), `reconnect(int)` (658), `stats_n()` (670),
`stats_name(int)` (673), `stats_value(int) const` (676), `tun_stats() const` (682),
`post_cc_msg(const std::string&)` (688).

---

## 2. Struct fields — `client/ovpncli.hpp`

### `Config` (extends `ConfigCommon`) — line 347
```cpp
struct Config : public ConfigCommon
{
    std::string content;        // ovpncli.hpp:350 — OpenVPN profile as a string
    // ... many more (key/value content, proto override, etc.)
};
```
The shim sets `config.content = <full .ovpn text>`.

### `EvalConfig` — line 54
```cpp
struct EvalConfig
{
    bool error = false;                 // ovpncli.hpp:57  — true if error
    std::string message;                // ovpncli.hpp:60  — if error, message here
    std::string userlockedUsername;     // 63
    std::string profileName;            // 66
    std::string friendlyName;           // 69
    bool autologin = false;             // 72  — false => username/password required
    // ... more
};
```

### `ProvideCreds` — line 113
```cpp
struct ProvideCreds
{
    std::string username;       // ovpncli.hpp:115
    std::string password;       // ovpncli.hpp:116
    std::string http_proxy_user;// 118
    std::string http_proxy_pass;// 119
    std::string response;       // 122 — challenge response
    // ... more
};
```

### `Event` — line 386
```cpp
struct Event
{
    bool error = false; // ovpncli.hpp:388 — true if error (fatal or nonfatal)
    bool fatal = false; // ovpncli.hpp:389 — true if fatal (will disconnect)
    std::string name;   // ovpncli.hpp:390 — event name
    std::string info;   // ovpncli.hpp:391 — additional event info
};
```
> Field ORDER is `error, fatal, name, info` (error/fatal first). All four assumed
> fields exist with the assumed names. Matches assumptions.

### `Status` — line 428
```cpp
struct Status
{
    bool error = false;  // ovpncli.hpp:430 — true if error
    std::string status;  // ovpncli.hpp:431 — optional short error label
    std::string message; // ovpncli.hpp:432 — if error, message here
};
```
> **[DIFF]** task asked for `error` + `message`; there is also an extra
> `std::string status` field (short label) BETWEEN them. `connect()` and
> `provide_creds()` both return this `Status`.

### `LogInfo` — line 437
```cpp
struct LogInfo
{
    LogInfo();
    LogInfo(std::string str);   // ctor moves into text
    std::string text;           // ovpncli.hpp:446 — log output (usually one line)
};
```

### `TransportStats` — line 468
```cpp
struct TransportStats
{
    long long bytesIn;          // ovpncli.hpp:470
    long long bytesOut;         // ovpncli.hpp:471
    long long packetsIn;        // ovpncli.hpp:472
    long long packetsOut;       // ovpncli.hpp:473
    // + lastPacketReceived (binary-ms) below
};
```
> **[CONFIRMED — matches first assumption]** byte counters are **`bytesIn` / `bytesOut`**
> (camelCase), type **`long long`** (NOT `bytes_in`/`bytes_out`, NOT `uint64_t`).
> Also note the related `InterfaceStats` (line 457) and `OpenVPNClient::tun_stats()`
> use the SAME `bytesIn`/`bytesOut` naming.

---

## 3. `TunBuilderBase` virtual methods — `openvpn/tun/builder/base.hpp`

`OpenVPNClient` inherits `TunBuilderBase`, so override these on the same subclass.
All callbacks are non-pure virtuals with default bodies returning `false`/`-1`, so a
shim only overrides what it needs. Call ORDER per the header doc: `tun_builder_new()`
first, setters in the middle, `tun_builder_establish()` last.

```cpp
// base.hpp:50
virtual bool tun_builder_new();

// base.hpp:67  — OSI layer: 3=TUN (only supported), 2=TAP, 0. (default returns true)
virtual bool tun_builder_set_layer(int layer);

// base.hpp:83
virtual bool tun_builder_set_remote_address(const std::string &address, bool ipv6);

// base.hpp:101-105  (multi-line signature — note 5 params incl. net30)
virtual bool tun_builder_add_address(const std::string &address,
                                     int prefix_length,
                                     const std::string &gateway, // optional
                                     bool ipv6,
                                     bool net30);

// base.hpp:139-141
virtual bool tun_builder_reroute_gw(bool ipv4,
                                    bool ipv6,
                                    unsigned int flags);

// base.hpp:158-161
virtual bool tun_builder_add_route(const std::string &address,
                                   int prefix_length,
                                   int metric,
                                   bool ipv6);

// base.hpp:178-181
virtual bool tun_builder_exclude_route(const std::string &address,
                                       int prefix_length,
                                       int metric,
                                       bool ipv6);

// base.hpp:195  — *** DNS ENTRY POINT ***
virtual bool tun_builder_set_dns_options(const DnsOptions &dns);

// base.hpp:211
virtual bool tun_builder_set_mtu(int mtu);

// base.hpp:226
virtual bool tun_builder_set_session_name(const std::string &name);

// base.hpp:361  — returns the tun fd (caller owns it); -1 on failure. Called LAST.
virtual int tun_builder_establish();          // [DIFF] returns int (the fd), NOT bool/void

// base.hpp:406  — reconnect with persisted TUN state.
virtual void tun_builder_establish_lite();

// base.hpp:417  — teardown; disconnect=true means pre-final-disconnect.
virtual void tun_builder_teardown(bool disconnect);
```

### **[DIFF — IMPORTANT for T7/T8]** DNS API
There is **NO `tun_builder_add_dns_server`** in this tag. DNS is configured via a
SINGLE call to **`tun_builder_set_dns_options(const DnsOptions &dns)`** (base.hpp:195).
Per the doc comment, this "is called only once and overrides when called multiple times"
(base.hpp:189). `DnsOptions` is defined in **`openvpn/client/dns_options.hpp`**
(included at base.hpp:22). The shim must walk the `DnsOptions` struct (servers list,
search domains, etc.) rather than receiving server-by-server callbacks.

### `reroute_gw` is present (base.hpp:139) — used to know whether to claim the default route.

Other TunBuilderBase callbacks available (not required for v0):
`tun_builder_persist()` (379), `tun_builder_get_local_networks(bool ipv6)` (395).

---

## 4. Summary of DIFFs from original task assumptions

| Assumption in task | REAL in `release/3.11.6` |
|---|---|
| `tun_builder_add_dns_server` | **GONE** — use `tun_builder_set_dns_options(const DnsOptions&)` (base.hpp:195) |
| `tun_builder_establish` return type unspecified | returns **`int`** (the tun fd; -1 on failure) (base.hpp:361) |
| `connect()` return type unspecified | returns **`Status`** (ovpncli.hpp:631) |
| `Status{error,message}` | also has middle field **`std::string status`** (label) (ovpncli.hpp:431) |
| TransportStats byte fields | **`bytesIn`/`bytesOut`**, type `long long` — confirmed (ovpncli.hpp:470-471) |
| `tun_builder_add_address(address,prefix,ipv6)` | actually 5 params: `+ const std::string &gateway, + bool net30` (base.hpp:101) |
| `tun_builder_add_route(address,prefix,ipv6)` | actually 4 params: `+ int metric` (base.hpp:158) |
| `log` is a plain virtual | declared `virtual void log(const LogInfo&) override = 0;` (overrides LogReceiver) (ovpncli.hpp:729) |
