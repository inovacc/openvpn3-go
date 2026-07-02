package openvpn3

// EventSink receives streamed OpenVPN3 events and log lines. It is declared in
// an UNTAGGED file so both the stub build (//go:build !openvpn3) and the cgo
// build (//go:build openvpn3 && windows && cgo) share one definition. The cgo
// callbacks (callbacks.go) invoke these methods from the C event/log
// trampolines; implementations MUST be safe to call from a non-Go-created
// thread (cgo will have attached one) and SHOULD return quickly — the C side
// owns the passed strings only for the duration of the call.
type EventSink interface {
	OnEvent(e Event)
	OnLog(s string)
}
