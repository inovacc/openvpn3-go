//go:build windows && cgo

// parity_emit_wire_test.go — in-package half of the differential-parity emitter.
//
// `helperOpCmd` / `helperOpResp` are UNEXPORTED, so the `go run` emitter
// (parity_emit.go) cannot reach them. This test file lives in package openvpn3
// and marshals the REAL wire structs. Run with:
//
//	go test -run TestParityEmitWire -v .
//
// Every emitted line is prefixed `@@ ` so it can be sieved out of `go test -v`
// framing noise.
package openvpn3

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func pesc(s string) string {
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

func TestParityEmitWire(t *testing.T) {
	type vec struct {
		id   string
		op   string
		args map[string]string
	}
	vecs := []vec{
		{"w01_no_args", "tun-close", nil},
		{"w02_empty_args", "tun-close", map[string]string{}},
		{"w03_open_full", "tun-open", map[string]string{"address": "10.8.0.2", "prefix": "24", "gateway": "10.8.0.1", "mtu": "1400"}},
		{"w04_route", "add-route", map[string]string{"address": "0.0.0.0", "prefix": "0", "metric": "0"}},
		{"w05_dns", "set-dns", map[string]string{"servers": "1.1.1.1,8.8.8.8"}},
		{"w06_empty_op", "", map[string]string{"k": "v"}},
		{"w07_unicode", "トンネル", map[string]string{"キー": "値"}},
		{"w08_html_chars", "tun-open", map[string]string{"address": "a<b>c&d", "note": "x<y"}},
		{"w09_quotes_backslash", "tun-open", map[string]string{"p": `he said "hi"\n`}},
		{"w10_newline_value", "tun-open", map[string]string{"p": "line1\nline2"}},
		{"w11_empty_value", "tun-open", map[string]string{"empty": ""}},
		{"w12_line_sep", "tun-open", map[string]string{"p": "a b c"}},
		{"w13_many_keys", "tun-open", map[string]string{"z": "1", "a": "2", "m": "3", "B": "4"}},
		{"w14_emoji", "tun-open", map[string]string{"e": "\U0001f600"}},
	}

	for _, v := range vecs {
		cmd := helperOpCmd{Op: v.op, Args: v.args}
		b, err := json.Marshal(cmd)
		if err != nil {
			fmt.Printf("@@ WIRE %s MARSHAL_ERROR %s\n", v.id, pesc(err.Error()))
			continue
		}
		fmt.Printf("@@ WIRE %s %s\n", v.id, pesc(string(b)))
	}

	// helperOpResp decode shapes.
	respVecs := []struct{ id, raw string }{
		{"r01_ok", `{"ok":true}`},
		{"r02_err", `{"ok":false,"error":"boom"}`},
		{"r03_empty", `{}`},
		{"r04_extra", `{"ok":true,"error":"","extra":1}`},
		{"r05_unicode_err", `{"ok":false,"error":"ошибка"}`},
	}
	for _, v := range respVecs {
		var r helperOpResp
		if err := json.Unmarshal([]byte(v.raw), &r); err != nil {
			fmt.Printf("@@ RESP %s DECODE_ERROR %s\n", v.id, pesc(err.Error()))
			continue
		}
		fmt.Printf("@@ RESP %s ok=%v error=%s\n", v.id, r.OK, pesc(r.Error))
	}

	// Error-string formatting from sendHelperRequests. The pipe I/O cannot run
	// without an elevated helper, so the three fmt.Errorf FORMATS are reproduced
	// verbatim from tunbridge_windows.go:136,141,144 — flagged as
	// format-replicated (not an API-level oracle) in PORT-TRACK.md.
	opVecs := []struct{ id, op, detail string }{
		{"f01_ascii", "tun-open", "x"},
		{"f02_empty_op", "", "x"},
		{"f03_unicode_op", "トンネル", "x"},
		{"f04_quote_op", `he "q" op`, "x"},
		{"f05_backslash_op", `a\b`, "x"},
		{"f06_empty_detail", "tun-open", ""},
		{"f07_unicode_detail", "tun-open", "ошибка"},
	}
	for _, v := range opVecs {
		fmt.Printf("@@ HELPERFMT %s failed=%s\n", v.id,
			pesc(fmt.Errorf("openvpn3: helper op %q failed: %s", v.op, v.detail).Error()))
		fmt.Printf("@@ HELPERFMT %s send=%s\n", v.id,
			pesc(fmt.Errorf("openvpn3: send helper op %q: %s", v.op, v.detail).Error()))
		fmt.Printf("@@ HELPERFMT %s read=%s\n", v.id,
			pesc(fmt.Errorf("openvpn3: read helper resp for op %q: %s", v.op, v.detail).Error()))
	}
	fmt.Printf("@@ PIPEPATH %s\n", pesc(helperPipePath))
}
