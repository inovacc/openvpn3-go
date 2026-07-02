# Roadmap

## Current Status
**Overall Progress:** 10% - Scaffold + reusable ovpn3 facade in place

## Phases

### Phase 1: Foundation [IN PROGRESS]
- [x] Project scaffold (structure, tooling, CI)
- [x] Reusable hardened `pkg/ovpn3` facade (stub build, validation, lifecycle)
- [x] Daemon ↔ session adapter (`internal/vpnsvc`)
- [ ] Land native C/C++ sources in `pkg/ovpn3/native`

### Phase 2: Native integration [NOT STARTED]
Driven by docs/INTEGRATION.md, one increment per step.
- [ ] Wire cgo bridge (`client_cgo.go` preamble, `CGO_ENABLED=1` builds)
- [ ] Map ClientAPI → facade: Connect
- [ ] Map ClientAPI → facade: Disconnect / Status / events
- [ ] Migrate existing Go (profile parsing, transport) into `internal/`
- [ ] Feed real `ovpn3.Config` from app config into the daemon

### Phase 3: Polish & Release [NOT STARTED]
- [ ] End-to-end session test against a real server
- [ ] v1.0.0 release
