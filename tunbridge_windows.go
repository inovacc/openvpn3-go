//go:build windows && cgo

// INERT on Windows — OpenVPN3 3.11.6 does not invoke TunBuilderBase on Windows
// (USE_TUN_BUILDER is Android/iOS-only), so these TunBuilder-translation helpers
// are not exercised by the Windows OpenVPN3 path. Retained for non-Windows/future
// use and unit-tested in isolation.

package openvpn3

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// tunBridge accumulates TunBuilder verbs (via spec) and, on Establish, renders
// them into ordered HelperRequests and ships them to lensr's elevated VPN
// helper over its named pipe. Teardown sends the inverse close op.
//
// DESIGN DECISION — self-contained pipe client (NOT internal/manager adapter):
//
//	pkg/* MUST NOT import internal/manager (layering: internal/ is private to
//	the module's app layer; a pkg/ leaf importing it inverts the dependency and
//	would also create an import cycle once manager wires the connector). So this
//	file speaks the vpnhelper wire protocol DIRECTLY: a length-agnostic JSON
//	request/response over the same named pipe (\\.\pipe\lensr_vpn_helper) using
//	the same json.Encoder/json.Decoder framing as
//	internal/manager/vpnhelper_client_windows.go. The proto structs are
//	replicated here (helperOpCmd/helperOpResp) so the two stay wire-compatible.
//
// GAP — granular tun verbs are not yet a server-side contract:
//
//	The CURRENT elevated helper (internal/manager/vpnhelper_proto_windows.go)
//	exposes only HIGH-LEVEL ops: "connect" (with config_path), "disconnect",
//	"status", plus the CDP/MSI ops. It does NOT yet implement the GRANULAR
//	TunBuilder ops this bridge emits ("tun-open", "add-route", "set-dns",
//	"tun-close" — see TunSpec.HelperRequests()). Those granular verbs are
//	required for the OpenVPN3-in-process path (where lensr owns the protocol
//	and only delegates the privileged TUN/route/DNS mutations) but the helper
//	server has not been extended for them yet.
//
//	Therefore Establish/Teardown are written against the FUTURE granular
//	contract and are wire-ready, but will fail at runtime until the helper
//	gains the matching op handlers. TunSpec.HelperRequests() is intentionally
//	left UNCHANGED (its op names "tun-open"/"add-route"/"set-dns" define the
//	target contract; changing them now would only churn its tests without a
//	real server to reconcile against). When the helper grows these verbs, keep
//	the names in sync across HelperRequests(), this file, and the proto.
type tunBridge struct {
	spec    TunSpec
	applied bool
}

func newTunBridge() *tunBridge { return &tunBridge{} }

// helperPipePath mirrors internal/manager/vpnhelper_proto_windows.go.
const helperPipePath = `\\.\pipe\lensr_vpn_helper`

// helperOpCmd is the wire request. It is a superset that carries both the
// existing high-level fields and the granular tun fields keyed by Op/Args, so
// it is forward-compatible with the helper proto. JSON tags match the proto.
type helperOpCmd struct {
	Op   string            `json:"op"`
	Args map[string]string `json:"args,omitempty"`
}

// helperOpResp mirrors the helper's response envelope (ok/error).
type helperOpResp struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Establish renders the accumulated spec and sends each request to the helper.
// Idempotent guard: a second Establish without a Teardown is a no-op success.
func (b *tunBridge) Establish() error {
	if b.applied {
		return nil
	}
	reqs := b.spec.HelperRequests()
	if len(reqs) == 0 {
		// Nothing to apply (no addresses/routes/dns were pushed). Treat as a
		// successful no-op so the C side gets a clean establish.
		b.applied = true
		return nil
	}
	if err := sendHelperRequests(reqs); err != nil {
		return err
	}
	b.applied = true
	return nil
}

// Teardown sends the close op so the helper removes routes/DNS and drops the
// TUN device. Safe to call when nothing was applied.
func (b *tunBridge) Teardown() error {
	if !b.applied {
		return nil
	}
	b.applied = false
	return sendHelperRequests([]HelperRequest{{Op: "tun-close"}})
}

// sendHelperRequests opens the helper named pipe and sends each HelperRequest
// as one JSON message, reading a response per request. It uses the same
// CreateFile + json.Encoder/Decoder framing as the manager-side HelperClient so
// the two are wire-compatible. The connection is closed after the batch, which
// (per the helper protocol) signals end-of-session to the helper.
func sendHelperRequests(reqs []HelperRequest) error {
	pipePtr, err := windows.UTF16PtrFromString(helperPipePath)
	if err != nil {
		return fmt.Errorf("openvpn3: helper pipe path: %w", err)
	}

	h, err := windows.CreateFile(
		pipePtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil,
		windows.OPEN_EXISTING,
		0, 0,
	)
	if err != nil {
		return fmt.Errorf("openvpn3: open helper pipe (is the elevated vpnhelper running?): %w", err)
	}

	f := os.NewFile(uintptr(h), "lensr-vpnhelper-pipe")
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	dec := json.NewDecoder(f)

	for _, req := range reqs {
		cmd := helperOpCmd{Op: req.Op, Args: req.Args} //nolint:staticcheck // explicit field map: HelperRequest and helperOpCmd are distinct wire types
		if err := enc.Encode(cmd); err != nil {
			return fmt.Errorf("openvpn3: send helper op %q: %w", req.Op, err)
		}

		var resp helperOpResp
		if err := dec.Decode(&resp); err != nil {
			return fmt.Errorf("openvpn3: read helper resp for op %q: %w", req.Op, err)
		}
		if !resp.OK {
			return fmt.Errorf("openvpn3: helper op %q failed: %s", req.Op, resp.Error)
		}
	}
	return nil
}
