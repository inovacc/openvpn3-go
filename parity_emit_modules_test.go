//go:build !cgo

// parity_emit_modules_test.go — in-package half of the differential-parity
// emitter for the pure-logic / stub modules (sink, observers, tunnel,
// client_stub, engine_stub). Tagged `!cgo` so the STUB / ENGINESTUB tags always
// reference the stub implementations (stub.go / engine_stub.go), exactly
// mirroring the Rust `#[cfg(not(feature = "engine"))]` gate on the matching
// emission in examples/parity_golden.rs.
//
// emitEvent/emitLog are UNEXPORTED, so this must live in-package. Run with:
//
//	$env:CGO_ENABLED = "0"; go test -run TestParityEmitModules -v .
//
// Every emitted line is prefixed `@@ ` so it can be sieved out of `go test -v`
// framing noise.
package openvpn3

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func mesc(s string) string {
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

func b2i(v bool) int {
	if v {
		return 1
	}
	return 0
}

// recordingSink is the Go analogue of the Rust RecordingSink.
type recordingSink struct {
	events []Event
	logs   []string
}

func (r *recordingSink) OnEvent(e Event) { r.events = append(r.events, e) }
func (r *recordingSink) OnLog(s string)  { r.logs = append(r.logs, s) }

func TestParityEmitModules(t *testing.T) {
	out := func(format string, a ...any) { fmt.Printf("@@ "+format+"\n", a...) }

	// ── SINK ────────────────────────────────────────────────────────────────
	evIn := []Event{
		{Name: "RESOLVE"},
		{Name: "CONNECTING", Info: "vpn.example.com"},
		{Name: "AUTH_FAILED", Info: "bad creds", Error: true},
		{Error: true, Fatal: true},
		{Name: "ПОДКЛЮЧЕНО", Info: "サーバ ✓"},
		{Name: "N", Info: "line1\nline2"},
		{Name: "N", Info: "a<b>&c"},
		{Name: "N", Info: "\U0001f600"},
	}
	logIn := []string{"hello", "", "лог ✓", "a\nb", "\U0001f600", "   "}

	rec := &recordingSink{}
	var sink EventSink = rec
	for _, e := range evIn {
		sink.OnEvent(e)
	}
	for _, s := range logIn {
		sink.OnLog(s)
	}
	out("SINK events n=%d", len(rec.events))
	for i, e := range rec.events {
		out("SINK evt#%d name=%s;info=%s;error=%d;fatal=%d",
			i, mesc(e.Name), mesc(e.Info), b2i(e.Error), b2i(e.Fatal))
	}
	out("SINK logs n=%d", len(rec.logs))
	for i, s := range rec.logs {
		out("SINK log#%d %s", i, mesc(s))
	}
	// Go's absent sink is a nil interface the callbacks nil-check; it delivers
	// nothing (the Rust NoopSink analogue). Constant on both sides.
	out("SINK noop delivered=0")

	// ── OBS ─────────────────────────────────────────────────────────────────
	var seenEv []string
	SetEventHandler(func(e Event) { seenEv = append(seenEv, "a:"+e.Name) })
	emitEvent(Event{Name: "ONE"})
	SetEventHandler(func(e Event) { seenEv = append(seenEv, "b:"+e.Name) })
	emitEvent(Event{Name: "TWO"})
	SetEventHandler(nil)
	emitEvent(Event{Name: "THREE"}) // noop
	SetEventHandler(func(e Event) { seenEv = append(seenEv, "c:"+e.Name) })
	emitEvent(Event{Name: "ПОДКЛЮЧЕНО"})
	SetEventHandler(func(e Event) {
		seenEv = append(seenEv, "r:"+e.Name)
		SetEventHandler(nil) // re-enter registry from inside the handler
	})
	emitEvent(Event{Name: "RE1"})
	emitEvent(Event{Name: "RE2"}) // noop (cleared)
	SetEventHandler(nil)
	out("OBS event n=%d", len(seenEv))
	for i, s := range seenEv {
		out("OBS event#%d %s", i, mesc(s))
	}

	var seenLog []string
	SetLogHandler(func(s string) { seenLog = append(seenLog, "a:"+s) })
	emitLog("one")
	SetLogHandler(func(s string) { seenLog = append(seenLog, "b:"+s) })
	emitLog("two")
	SetLogHandler(nil)
	emitLog("three") // noop
	SetLogHandler(func(s string) { seenLog = append(seenLog, "c:"+s) })
	emitLog("лог")
	SetLogHandler(func(s string) {
		seenLog = append(seenLog, "r:"+s)
		SetLogHandler(nil)
	})
	emitLog("re1")
	emitLog("re2") // noop
	SetLogHandler(nil)
	out("OBS log n=%d", len(seenLog))
	for i, s := range seenLog {
		out("OBS log#%d %s", i, mesc(s))
	}

	// ── TUNNEL ────────────────────────────────────────────────────────────────
	tun := Default()
	cfg := ConnectInput{ConfigContent: "client\n"}
	out("TUNNEL version %s", mesc(Version))
	out("TUNNEL states %s;%s;%s;%s",
		mesc(StateDisconnected), mesc(StateConnecting), mesc(StateConnected), mesc(StateError))
	st := tun.Status()
	out("TUNNEL default_status state=%s;error=%s", mesc(st.State), mesc(st.Error))
	if _, err := tun.Connect(context.Background(), cfg); err != nil {
		out("TUNNEL default_connect ok=0;err=%s", mesc(err.Error()))
	} else {
		out("TUNNEL default_connect ok=1;err=")
	}
	out("TUNNEL default_disconnect ok=%d", b2i(tun.Disconnect(SessionHandle{}) == nil))
	out("TUNNEL default_close ok=%d", b2i(tun.Close() == nil))

	// ── STUB (client_stub) ──────────────────────────────────────────────────
	if _, err := New(Config{Content: "client\n"}, nil); err != nil {
		out("STUB new ok=0;err=%s", mesc(err.Error()))
	} else {
		out("STUB new ok=1;err=")
	}
	var c *Client // nil receiver — legal in Go, mirrored by Rust's Client::default() (D3)
	if err := c.Connect(context.Background()); err != nil {
		out("STUB connect ok=0;err=%s", mesc(err.Error()))
	} else {
		out("STUB connect ok=1;err=")
	}
	if err := c.Stop(); err != nil {
		out("STUB stop ok=0;err=%s", mesc(err.Error()))
	} else {
		out("STUB stop ok=1;err=")
	}
	if _, err := c.TransportStats(); err != nil {
		out("STUB transport_stats ok=0;err=%s", mesc(err.Error()))
	} else {
		out("STUB transport_stats ok=1;err=")
	}

	// ── ENGINESTUB (engine_stub package-level fns) ──────────────────────────
	if _, err := Connect(context.Background(), ConnectInput{}); err != nil {
		out("ENGINESTUB connect ok=0;err=%s", mesc(err.Error()))
	} else {
		out("ENGINESTUB connect ok=1;err=")
	}
	out("ENGINESTUB disconnect ok=%d", b2i(Disconnect(SessionHandle{}) == nil))
	es := Status()
	out("ENGINESTUB status state=%s;error=%s", mesc(es.State), mesc(es.Error))
	out("ENGINESTUB close ok=%d", b2i(Close() == nil))
	out("ENGINESTUB available %d", b2i(Available()))
	SetLogger(nil)
	SetLogger(nil)
	out("ENGINESTUB set_logger ok=1")
}
