# Roadmap

## Current Status
**Overall Progress:** Vendored-source reusable module; cgo engine builds via plain go get + C toolchain

## Phases

### Phase 1: Foundation [IN PROGRESS]
- [x] Project scaffold (structure, tooling, CI)
- [x] Reusable hardened `openvpn3` facade (stub build, validation, lifecycle)
- [x] Daemon ↔ session adapter (`cmd/openvpn`)
- [x] Land native C/C++ sources (vendored at module root — supersedes the old pkg/ovpn3/native layout)

### Phase 2: Native integration [NOT STARTED]
Driven by docs/INTEGRATION.md, one increment per step.
- [x] Wire cgo bridge (`cgo_flags.go` / `engine_windows.go`, `CGO_ENABLED=1` builds)
- [ ] Map ClientAPI → facade: Connect
- [ ] Map ClientAPI → facade: Disconnect / Status / events
- [ ] Migrate existing Go (profile parsing, transport) into the flat package root (`profile.go`, `tunspec.go`)
- [ ] Feed real `openvpn3.ConnectInput` / `Config` from app config into `cmd/openvpn`

### Phase 3: Polish & Release [NOT STARTED]
- [ ] End-to-end session test against a real server
- [ ] v1.0.0 release
