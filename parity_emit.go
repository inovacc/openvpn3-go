//go:build ignore

// parity_emit.go — differential-parity golden emitter for the Rust port.
//
// Exercises the REAL exported openvpn3 Go API and prints one deterministic,
// parseable line per vector. The matching Rust emitter is
// D:\lensr-rs\crates\openvpn3\examples\parity_golden.rs.
//
//	go run parity_emit.go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"

	openvpn3 "github.com/inovacc/openvpn3-go"
)

// esc is a byte-exact, language-neutral escaper: backslash-doubled, printable
// ASCII verbatim, everything else \xHH over the UTF-8 bytes. Deliberately NOT
// %q — Go's %q and Rust's {:?} diverge on non-ASCII (deviation D39/D47/D49).
func esc(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			b.WriteString(`\\`)
		case c >= 0x20 && c <= 0x7e:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, `\x%02x`, c)
		}
	}
	return b.String()
}

// escCap escapes, but collapses long values to a length+sha256 fingerprint so a
// 10k-char vector stays one readable line while still being byte-sensitive.
func escCap(s string) string {
	if len(s) <= 200 {
		return esc(s)
	}
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("<len=%d sha256=%s>", len(s), hex.EncodeToString(sum[:8]))
}

func out(format string, a ...any) { fmt.Printf(format+"\n", a...) }

// ── vectors ────────────────────────────────────────────────────────────────

type profVec struct {
	id      string
	content string
	user    string
	pass    string
}

func profileVectors() []profVec {
	long := strings.Repeat("client\n", 2000)
	return []profVec{
		{"p01_empty", "", "", ""},
		{"p02_spaces_nl", "   \n", "", ""},
		{"p03_tab_cr_nl", "\t\r\n ", "", ""},
		{"p04_nbsp", "\u00a0", "", ""},
		{"p05_zwsp", "\u200b", "", ""},
		{"p06_ideographic_space", "\u3000", "", ""},
		{"p07_vertical_tab", "\v", "", ""},
		{"p08_formfeed", "\f", "", ""},
		{"p09_nel", "\u0085", "", ""},
		{"p10_ogham_space", "\u1680", "", ""},
		{"p11_simple", "client\n", "u", "p"},
		{"p12_untrimmed", "  client\n ", "u", "p"},
		{"p13_crlf", "client\r\nremote vpn.example.com 1194\r\ndev tun\r\n", "u", "p"},
		{"p14_unicode", "\u043a\u043b\u0438\u0435\u043d\u0442\n# \u30b3\u30e1\u30f3\u30c8 \u2713\nremote vpn.example.com 1194\n", "\u00fcser", "p\u00e4ss"},
		{"p15_inline_blocks", "client\ndev tun\n<ca>\n-----BEGIN CERTIFICATE-----\nMIIBxyz\n-----END CERTIFICATE-----\n</ca>\n<tls-auth>\nffffffff\n</tls-auth>\nkey-direction 1\n", "u", "p"},
		{"p16_nul_byte", "client\x00\n", "", ""},
		{"p17_long", long, "u", "p"},
		{"p18_empty_creds_nonempty_content", "client\n", "", ""},
		{"p19_only_newlines", "\n\n\n", "", ""},
		{"p20_emoji", "client\n# \U0001f600\n", "\U0001f600", "\U0001f600"},
	}
}

type tunOp struct {
	kind    string // addr | route | dns | mtu | remote
	s1, s2  string
	i1, i2  int
	b1      bool
}

type tunVec struct {
	id  string
	ops []tunOp
}

func tunspecVectors() []tunVec {
	var manyRoutes []tunOp
	for i := 0; i < 10; i++ {
		manyRoutes = append(manyRoutes, tunOp{kind: "route", s1: fmt.Sprintf("10.%d.0.0", i), i1: 16, i2: i})
	}
	return []tunVec{
		{"t01_empty", nil},
		{"t02_only_dns", []tunOp{{kind: "dns", s1: "1.1.1.1"}}},
		{"t03_only_dns_x3", []tunOp{{kind: "dns", s1: "1.1.1.1"}, {kind: "dns", s1: "8.8.8.8"}, {kind: "dns", s1: "9.9.9.9"}}},
		{"t04_addr_mtu0", []tunOp{{kind: "addr", s1: "10.8.0.2", i1: 24, s2: "10.8.0.1"}, {kind: "mtu", i1: 0}}},
		{"t05_mtu_negative", []tunOp{{kind: "addr", s1: "10.8.0.2", i1: 24, s2: "10.8.0.1"}, {kind: "mtu", i1: -1}}},
		{"t06_mtu_one", []tunOp{{kind: "addr", s1: "10.8.0.2", i1: 24, s2: "10.8.0.1"}, {kind: "mtu", i1: 1}}},
		{"t07_full", []tunOp{
			{kind: "remote", s1: "203.0.113.5"},
			{kind: "addr", s1: "10.8.0.2", i1: 24, s2: "10.8.0.1"},
			{kind: "addr", s1: "10.8.0.3", i1: 24, s2: "10.8.0.1"},
			{kind: "route", s1: "10.0.0.0", i1: 8, i2: 5},
			{kind: "route", s1: "0.0.0.0", i1: 0, i2: 0},
			{kind: "dns", s1: "1.1.1.1"},
			{kind: "dns", s1: "8.8.8.8"},
			{kind: "mtu", i1: 1400},
		}},
		{"t08_empty_gateway", []tunOp{{kind: "addr", s1: "10.8.0.2", i1: 24, s2: ""}}},
		{"t09_route_0_0_0_0_prefix1", []tunOp{{kind: "route", s1: "0.0.0.0", i1: 1, i2: 0}}},
		{"t10_route_v6_default", []tunOp{{kind: "route", s1: "::", i1: 0, i2: 0, b1: true}}},
		{"t11_route_default_gw", []tunOp{{kind: "route", s1: "0.0.0.0", i1: 0, i2: 0}}},
		{"t12_dns_with_comma", []tunOp{{kind: "dns", s1: "1.1.1.1,8.8.8.8"}, {kind: "dns", s1: "9.9.9.9"}}},
		{"t13_negative_prefix_metric", []tunOp{{kind: "addr", s1: "10.8.0.2", i1: -5, s2: "gw"}, {kind: "route", s1: "10.0.0.0", i1: -8, i2: -3}}},
		{"t14_many_routes", manyRoutes},
		{"t15_addr_only_mtu9000", []tunOp{{kind: "addr", s1: "10.8.0.2", i1: 32, s2: "10.8.0.1"}, {kind: "mtu", i1: 9000}}},
		{"t16_dns_empty_string", []tunOp{{kind: "dns", s1: ""}}},
		{"t17_dns_unicode", []tunOp{{kind: "dns", s1: "dns.\u00e9xample"}, {kind: "dns", s1: "\u30b5\u30fc\u30d0"}}},
		{"t18_ipv6_addr_first", []tunOp{{kind: "addr", s1: "fd00::2", i1: 64, s2: "fd00::1", b1: true}, {kind: "addr", s1: "10.8.0.2", i1: 24, s2: "10.8.0.1"}, {kind: "mtu", i1: 1280}}},
		{"t19_dup_default_routes", []tunOp{{kind: "route", s1: "0.0.0.0", i1: 0, i2: 0}, {kind: "route", s1: "0.0.0.0", i1: 0, i2: 1}}},
		{"t20_maxint_mtu", []tunOp{{kind: "mtu", i1: 2147483647}}},
	}
}

func buildSpec(ops []tunOp) *openvpn3.TunSpec {
	s := &openvpn3.TunSpec{}
	for _, o := range ops {
		switch o.kind {
		case "remote":
			s.SetRemoteAddress(o.s1, o.b1)
		case "addr":
			s.AddAddress(o.s1, o.i1, o.s2, o.b1)
		case "route":
			s.AddRoute(o.s1, o.i1, o.i2, o.b1)
		case "dns":
			s.AddDNSServer(o.s1)
		case "mtu":
			s.SetMTU(o.i1)
		}
	}
	return s
}

func argsStr(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, esc(k)+"="+esc(m[k]))
	}
	return strings.Join(parts, ";")
}

func sessionKeyVectors() []struct{ id, path, content string } {
	return []struct{ id, path, content string }{
		{"k01_path_only", `C:\profiles\dev.ovpn`, ""},
		{"k02_simple", "", "client\n"},
		{"k03_crlf", "", "client\r\n"},
		{"k04_unicode", "", "\u043a\u043b\u0438\u0435\u043d\u0442\n"},
		{"k05_empty_both", "", ""},
		{"k06_long", "", strings.Repeat("client\n", 2000)},
		{"k07_path_and_content", `C:\p.ovpn`, "client\n"},
		{"k08_nul", "", "client\x00\n"},
		{"k09_emoji", "", "\U0001f600"},
	}
}

func main() {
	// ── error sentinels (bare + %w-wrapped) ───────────────────────────────
	sentinels := []struct {
		id  string
		err error
	}{
		{"e01_notbuilt", openvpn3.ErrOpenVPN3NotBuilt},
		{"e02_evalconfig", openvpn3.ErrEvalConfig},
		{"e03_connect", openvpn3.ErrConnect},
		{"e04_notrunning", openvpn3.ErrNotRunning},
		{"e05_unavailable", openvpn3.ErrUnavailable},
		{"e06_notimplemented", openvpn3.ErrNotImplemented},
	}
	for _, s := range sentinels {
		out("ERR %s bare=%s", s.id, esc(s.err.Error()))
	}
	details := []struct{ id, d string }{
		{"d01", "empty profile content"},
		{"d02", ""},
		{"d03", "\u043e\u0448\u0438\u0431\u043a\u0430 \u2713"},
		{"d04", "line1\nline2"},
		{"d05", `quoted "value" and \backslash`},
		{"d06", strings.Repeat("x", 300)},
	}
	for _, s := range sentinels {
		for _, d := range details {
			out("ERRWRAP %s/%s %s", s.id, d.id, escCap(fmt.Errorf("%w: %s", s.err, d.d).Error()))
		}
	}

	// ── profile.ConfigFromOVPN ────────────────────────────────────────────
	for _, v := range profileVectors() {
		cfg, err := openvpn3.ConfigFromOVPN(v.content, openvpn3.Credentials{User: v.user, Pass: v.pass})
		if err != nil {
			out("PROFILE %s ok=0 err=%s", v.id, escCap(err.Error()))
			continue
		}
		out("PROFILE %s ok=1 content=%s user=%s pass=%s",
			v.id, escCap(cfg.Content), esc(cfg.Username), esc(cfg.Password))
	}
	// The on-disk fixture, read exactly as profile_test.go does.
	if raw, err := os.ReadFile("testdata/sample.ovpn"); err == nil {
		cfg, err := openvpn3.ConfigFromOVPN(string(raw), openvpn3.Credentials{User: "u", Pass: "p"})
		if err != nil {
			out("PROFILE p99_sample ok=0 err=%s", escCap(err.Error()))
		} else {
			out("PROFILE p99_sample ok=1 content=%s user=%s pass=%s",
				escCap(cfg.Content), esc(cfg.Username), esc(cfg.Password))
		}
	} else {
		out("PROFILE p99_sample READ_ERROR")
	}

	// ── tunspec.HelperRequests ────────────────────────────────────────────
	for _, v := range tunspecVectors() {
		s := buildSpec(v.ops)
		reqs := s.HelperRequests()
		defgw := 0
		if s.RedirectsDefaultGateway() {
			defgw = 1
		}
		out("TUNSPEC %s defgw=%d n=%d", v.id, defgw, len(reqs))
		for i, r := range reqs {
			out("TUNREQ %s#%d op=%s args=[%s]", v.id, i, esc(r.Op), argsStr(r.Args))
		}
	}

	// ── runtime_dlls ──────────────────────────────────────────────────────
	dlls := openvpn3.RuntimeDLLs()
	out("DLLS n=%d", len(dlls))
	for i, d := range dlls {
		out("DLL %d %s", i, esc(d))
	}

	// ── engine_windows session key ────────────────────────────────────────
	// NOTE: engine_windows.go's sessionKey is computed inline inside
	// (*engine).connect, which is unreachable without a live C++ core. The
	// FORMULA is reproduced here verbatim from engine_windows.go:81-85.
	for _, v := range sessionKeyVectors() {
		key := v.path
		if v.content != "" {
			sum := sha256.Sum256([]byte(v.content))
			key = "content:" + hex.EncodeToString(sum[:8])
		}
		out("SESSIONKEY %s key=%s", v.id, esc(key))
	}

	out("VERSION %s", esc(openvpn3.Version))
}
