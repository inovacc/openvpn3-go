// parity_emit_cli_test.go — in-package half of the differential-parity emitter
// for the CLI's line/JSON formatting.
//
// printEvent/printLog are UNEXPORTED and print to stdout, so this test captures
// os.Stdout around each call and emits one labelled line per vector.
//
//	go test -run TestParityEmitCLI -v ./cmd/openvpn
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	openvpn3 "github.com/inovacc/openvpn3-go"
)

func cesc(s string) string {
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

// capture runs f with os.Stdout redirected and returns everything it printed,
// with the single trailing newline fmt.Println adds stripped.
func capture(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	f()
	_ = w.Close()
	os.Stdout = old
	return strings.TrimSuffix(<-done, "\n")
}

func TestParityEmitCLI(t *testing.T) {
	evs := []struct {
		id string
		e  openvpn3.Event
	}{
		{"c01_name_only", openvpn3.Event{Name: "CONNECTED"}},
		{"c02_with_info", openvpn3.Event{Name: "RESOLVE", Info: "vpn.example.com"}},
		{"c03_error", openvpn3.Event{Name: "AUTH_FAILED", Info: "bad creds", Error: true}},
		{"c04_fatal_and_error", openvpn3.Event{Name: "AUTH_FAILED", Info: "bad creds", Error: true, Fatal: true}},
		{"c05_fatal_only", openvpn3.Event{Name: "X", Fatal: true}},
		{"c06_error_no_info", openvpn3.Event{Name: "X", Error: true}},
		{"c07_all_empty", openvpn3.Event{}},
		{"c08_unicode", openvpn3.Event{Name: "ПОДКЛЮЧЕНО", Info: "サーバ ✓"}},
		{"c09_info_with_spaces", openvpn3.Event{Name: "N", Info: "  leading and trailing  "}},
		{"c10_info_newline", openvpn3.Event{Name: "N", Info: "line1\nline2"}},
		{"c11_html_chars", openvpn3.Event{Name: "N", Info: "a<b>c&d"}},
		{"c12_quotes", openvpn3.Event{Name: "N", Info: `he said "hi"`}},
		{"c13_backslash", openvpn3.Event{Name: "N", Info: `C:\path\to`}},
		{"c14_long_info", openvpn3.Event{Name: "N", Info: strings.Repeat("z", 150)}},
		{"c15_emoji", openvpn3.Event{Name: "N", Info: "\U0001f600"}},
		{"c16_tab_info", openvpn3.Event{Name: "N", Info: "a\tb"}},
		{"c17_name_with_space", openvpn3.Event{Name: "TWO WORDS", Info: "i"}},
		{"c18_line_sep", openvpn3.Event{Name: "N", Info: "a\u2028b\u2029c"}},
	}
	for _, v := range evs {
		e := v.e
		out := capture(t, func() { printEvent(e, false) })
		fmt.Printf("@@ EVT %s plain=%s\n", v.id, cesc(out))
		outJ := capture(t, func() { printEvent(e, true) })
		fmt.Printf("@@ EVT %s json=%s\n", v.id, cesc(outJ))
	}

	logs := []struct{ id, s string }{
		{"l01_simple", "hello"},
		{"l02_empty", ""},
		{"l03_unicode", "лог ✓"},
		{"l04_html", "a<b>&c"},
		{"l05_quotes", `he said "hi"`},
		{"l06_backslash", `C:\p`},
		{"l07_newline", "a\nb"},
		{"l08_tab", "a\tb"},
		{"l09_emoji", "\U0001f600"},
		{"l10_line_sep", "a\u2028b"},
		{"l11_spaces", "   "},
	}
	for _, v := range logs {
		s := v.s
		out := capture(t, func() { printLog(s, false) })
		fmt.Printf("@@ LOG %s plain=%s\n", v.id, cesc(out))
		outJ := capture(t, func() { printLog(s, true) })
		fmt.Printf("@@ LOG %s json=%s\n", v.id, cesc(outJ))
	}
}
