package openvpn3

import "sync"

// observers.go is the build-tag-free event/log observer seam. The real engine
// (engine_windows.go) fans every OpenVPN3 client event + log line to the
// registered handlers via emitEvent/emitLog; consumers (the CLI, lensr) register
// via SetEventHandler/SetLogHandler. Process-wide, matching the singleton engine.

var (
	obsMu        sync.RWMutex
	eventHandler func(Event)
	logHandler   func(string)
)

// SetEventHandler registers a callback invoked for every OpenVPN3 client event
// (CONNECTED, RECONNECTING, AUTH_FAILED, ...). Pass nil to clear.
//
// Threading: the callback runs on a cgo-attached, non-Go-created thread. Keep it
// fast and non-blocking, and NEVER re-enter the engine (Connect/Disconnect) from
// it — that risks a callback/engine deadlock.
func SetEventHandler(fn func(Event)) {
	obsMu.Lock()
	eventHandler = fn
	obsMu.Unlock()
}

// SetLogHandler registers a callback for raw OpenVPN3 log lines. Same threading
// contract as SetEventHandler. Pass nil to clear.
func SetLogHandler(fn func(string)) {
	obsMu.Lock()
	logHandler = fn
	obsMu.Unlock()
}

func emitEvent(e Event) {
	obsMu.RLock()
	fn := eventHandler
	obsMu.RUnlock()
	if fn != nil {
		fn(e)
	}
}

func emitLog(s string) {
	obsMu.RLock()
	fn := logHandler
	obsMu.RUnlock()
	if fn != nil {
		fn(s)
	}
}
