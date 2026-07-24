//go:build windows && cgo

// mock_pipe_differential_test.go — the Go half of the tunbridge PipeTransport
// mock-named-pipe differential (Rust counterpart:
// crates/openvpn3/tests/mock_pipe_differential.rs).
//
// It stands up a REAL named-pipe server on the REAL const path
// (\\.\pipe\lensr_vpn_helper) using golang.org/x/sys/windows, runs the UNEXPORTED
// sendHelperRequests through the SAME scenarios as the Rust test, RECORDS the raw
// request bytes the client wrote, and prints them as `@@ MOCKPIPE <id> REQ …`
// using the shared pesc() escaper (defined in parity_emit_wire_test.go).
//
//	go test -run TestMockPipeDifferential -v .
//
// The `@@ MOCKPIPE` lines are diffed BYTE-FOR-BYTE against the Rust side by
// .scripts/00-AF_run_mockpipe_differential.ps1. This file also asserts Go's own
// error strings so a regression on either side is caught locally too.
package openvpn3

import (
	"errors"
	"fmt"
	"testing"

	"golang.org/x/sys/windows"
)

// mockAct is one scripted action per client request the server reads.
type mockAct struct {
	respond []byte // reply bytes; nil ⇒ close silently (→ client EOF)
}

func respond(b []byte) mockAct { return mockAct{respond: b} }
func closeSilently() mockAct   { return mockAct{respond: nil} }

// startMockServer creates the pipe SYNCHRONOUSLY (so it exists before the client
// opens), then serves `actions` on a goroutine. The returned channel yields the
// raw REQUEST bytes recorded once the batch (or the client) ends.
func startMockServer(t *testing.T, actions []mockAct) <-chan []byte {
	t.Helper()

	namePtr, err := windows.UTF16PtrFromString(helperPipePath)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}
	h, err := windows.CreateNamedPipe(
		namePtr,
		windows.PIPE_ACCESS_DUPLEX,
		0,   // PIPE_TYPE_BYTE|PIPE_READMODE_BYTE|PIPE_WAIT are all 0
		255, // PIPE_UNLIMITED_INSTANCES
		4096, 4096, 0, nil,
	)
	if err != nil {
		t.Fatalf("CreateNamedPipe: %v", err)
	}

	out := make(chan []byte, 1)
	go func() {
		// Block until the client connects (or is already connected).
		if cerr := windows.ConnectNamedPipe(h, nil); cerr != nil &&
			!errors.Is(cerr, windows.ERROR_PIPE_CONNECTED) {
			// Best-effort: record nothing on a connect failure.
			_ = windows.CloseHandle(h)
			out <- nil
			return
		}

		var recorded []byte
		var leftover []byte
		for _, act := range actions {
			line, ok := readLine(h, &leftover)
			if !ok {
				break // client closed early
			}
			recorded = append(recorded, line...)
			if act.respond == nil {
				break // close silently → client sees EOF
			}
			writeAll(h, act.respond)
		}
		// Close GRACEFULLY — CloseHandle only, NOT DisconnectNamedPipe (which
		// would sever the pipe and make the client's read fail with
		// ERROR_PIPE_NOT_CONNECTED rather than the ERROR_BROKEN_PIPE that Go's
		// runtime maps to io.EOF). Keeps the EOF arm genuinely reachable and
		// symmetric with the Rust mock.
		_ = windows.CloseHandle(h)
		out <- recorded
	}()
	return out
}

// readLine reads bytes until (and including) the next '\n'. Returns the full line
// and true, or (nil,false) on EOF/broken pipe before any newline.
func readLine(h windows.Handle, leftover *[]byte) ([]byte, bool) {
	for {
		for i, b := range *leftover {
			if b == '\n' {
				line := append([]byte(nil), (*leftover)[:i+1]...)
				*leftover = append([]byte(nil), (*leftover)[i+1:]...)
				return line, true
			}
		}
		buf := make([]byte, 1024)
		var done uint32
		if err := windows.ReadFile(h, buf, &done, nil); err != nil || done == 0 {
			if len(*leftover) == 0 {
				return nil, false
			}
			line := *leftover
			*leftover = nil
			return line, true
		}
		*leftover = append(*leftover, buf[:done]...)
	}
}

func writeAll(h windows.Handle, b []byte) {
	off := 0
	for off < len(b) {
		var done uint32
		if err := windows.WriteFile(h, b[off:], &done, nil); err != nil || done == 0 {
			return
		}
		off += int(done)
	}
	_ = windows.FlushFileBuffers(h)
}

func mkReq(op string, args map[string]string) HelperRequest {
	return HelperRequest{Op: op, Args: args}
}

const okLine = "{\"ok\":true}\n"

func notOkLine(err string) string {
	return fmt.Sprintf("{\"ok\":false,\"error\":%q}\n", err)
}

func TestMockPipeDifferential(t *testing.T) {
	run := func(id string, reqs []HelperRequest, actions []mockAct) ([]byte, error) {
		ch := startMockServer(t, actions)
		err := sendHelperRequests(reqs)
		recorded := <-ch
		fmt.Printf("@@ MOCKPIPE %s REQ %s\n", id, pesc(string(recorded)))
		return recorded, err
	}

	// 1. single_ok
	{
		rec, err := run("single_ok",
			[]HelperRequest{mkReq("tun-open", map[string]string{"address": "10.8.0.2", "prefix": "24"})},
			[]mockAct{respond([]byte(okLine))})
		if err != nil {
			t.Fatalf("single_ok: unexpected err: %v", err)
		}
		want := "{\"op\":\"tun-open\",\"args\":{\"address\":\"10.8.0.2\",\"prefix\":\"24\"}}\n"
		if string(rec) != want {
			t.Fatalf("single_ok wire bytes:\n got %q\nwant %q", rec, want)
		}
	}

	// 2. batch_stop_on_2nd
	{
		rec, err := run("batch_stop_on_2nd",
			[]HelperRequest{
				mkReq("tun-open", map[string]string{"address": "10.8.0.2", "prefix": "24"}),
				mkReq("add-route", map[string]string{"address": "10.0.0.0", "prefix": "8", "metric": "5"}),
				mkReq("set-dns", map[string]string{"servers": "1.1.1.1"}),
			},
			[]mockAct{respond([]byte(okLine)), respond([]byte(notOkLine("route table busy"))), respond([]byte(okLine))})
		want := "openvpn3: helper op \"add-route\" failed: route table busy"
		if err == nil || err.Error() != want {
			t.Fatalf("batch_stop_on_2nd err:\n got %v\nwant %q", err, want)
		}
		wantBytes := "{\"op\":\"tun-open\",\"args\":{\"address\":\"10.8.0.2\",\"prefix\":\"24\"}}\n" +
			"{\"op\":\"add-route\",\"args\":{\"address\":\"10.0.0.0\",\"metric\":\"5\",\"prefix\":\"8\"}}\n"
		if string(rec) != wantBytes {
			t.Fatalf("batch_stop_on_2nd wire bytes:\n got %q\nwant %q", rec, wantBytes)
		}
	}

	// 3. novel_args (unicode + <>& → HTML-escaped by encoding/json)
	{
		rec, err := run("novel_args",
			[]HelperRequest{mkReq("tun-open", map[string]string{"name": "a<b>c&d日本語🚀", "suffix": "x&y<z>"})},
			[]mockAct{respond([]byte(okLine))})
		if err != nil {
			t.Fatalf("novel_args: unexpected err: %v", err)
		}
		want := "{\"op\":\"tun-open\",\"args\":{\"name\":\"a\\u003cb\\u003ec\\u0026d日本語🚀\"," +
			"\"suffix\":\"x\\u0026y\\u003cz\\u003e\"}}\n"
		if string(rec) != want {
			t.Fatalf("novel_args wire bytes:\n got %q\nwant %q", rec, want)
		}
	}

	// 4. helper_rejected
	{
		rec, err := run("helper_rejected",
			[]HelperRequest{mkReq("tun-open", nil)},
			[]mockAct{respond([]byte(notOkLine("x")))})
		want := "openvpn3: helper op \"tun-open\" failed: x"
		if err == nil || err.Error() != want {
			t.Fatalf("helper_rejected err:\n got %v\nwant %q", err, want)
		}
		if string(rec) != "{\"op\":\"tun-open\"}\n" {
			t.Fatalf("helper_rejected wire bytes: %q", rec)
		}
	}

	// 5. eof
	{
		rec, err := run("eof",
			[]HelperRequest{mkReq("tun-open", nil)},
			[]mockAct{closeSilently()})
		want := "openvpn3: read helper resp for op \"tun-open\": EOF"
		if err == nil || err.Error() != want {
			t.Fatalf("eof err:\n got %v\nwant %q", err, want)
		}
		if string(rec) != "{\"op\":\"tun-open\"}\n" {
			t.Fatalf("eof wire bytes: %q", rec)
		}
	}

	// 6. bad_json — only the PREFIX must match Rust (inner decoder text differs, D29)
	{
		rec, err := run("bad_json",
			[]HelperRequest{mkReq("tun-open", nil)},
			[]mockAct{respond([]byte("{\"ok\":tru\n"))})
		prefix := "openvpn3: read helper resp for op \"tun-open\": "
		if err == nil || len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
			t.Fatalf("bad_json err prefix mismatch: %v", err)
		}
		if string(rec) != "{\"op\":\"tun-open\"}\n" {
			t.Fatalf("bad_json wire bytes: %q", rec)
		}
	}
}
