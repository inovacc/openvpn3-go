# OpenVPN3 Attribution

`pkg/openvpn3/openvpn` is a git submodule of https://github.com/OpenVPN/openvpn3
pinned at tag `release/3.11.6`
(commit `5b7841a847619e9e1ba3f7371e0c9e2743383481`).

## License

The pinned OpenVPN3 core is dual-licensed under
**GNU Affero General Public License v3.0 only (AGPL-3.0-only) OR
Mozilla Public License v2.0 (MPL-2.0)**, with a special permission to link
against OpenSSL when using AGPLv3. See the submodule's own license files:

- `pkg/openvpn3/openvpn/LICENSE.md` (top-level summary + OpenSSL linking exception)
- `pkg/openvpn3/openvpn/LICENSES/AGPL-3.0-only.txt`
- `pkg/openvpn3/openvpn/LICENSES/MPL-2.0.txt`

> NOTE: this is the dual AGPL-3.0 / MPL-2.0 licensing used by the modern
> OpenVPN3 core — NOT the older proprietary "OpenVPN license". AGPL-3.0 in
> particular carries network-copyleft obligations.

## License election (lensr)

The OpenVPN3 core is offered as **"AGPL-3.0-only OR MPL-2.0"** — a disjunctive
dual license that lets the downstream consumer pick ONE arm. **lensr ELECTS the
MPL-2.0 arm** for its use, distribution, and linking of the OpenVPN3 core
(including this `pkg/openvpn3/` cgo binding and the C++ shim under
`pkg/openvpn3/shim/` that links against it).

What electing MPL-2.0 means:

- **File-level (weak) copyleft.** MPL-2.0 copyleft attaches per *file*. Only
  modifications to MPL-2.0-licensed source files must themselves be made
  available under MPL-2.0. lensr's own first-party Go/C/C++ files (the cgo
  binding, the shim, the helper-pipe bridge) are NOT MPL-2.0-licensed merely by
  linking — MPL-2.0 explicitly permits combining MPL files with files under
  other licenses in a "Larger Work".
- **NO network copyleft.** By choosing MPL-2.0 over AGPL-3.0, lensr is NOT
  subject to AGPL-3.0 §13's network/SaaS source-disclosure obligation. Serving
  lensr over a network does not trigger a source-availability requirement for
  the combined work.
- **Source availability for the MPL files themselves** must still be honored if
  lensr distributes built binaries: recipients must be able to obtain the
  corresponding MPL-2.0 source (the OpenVPN3 submodule pinned above + any local
  modifications to its files). lensr makes no modifications to the submodule's
  files; the pinned upstream tag is the corresponding source.

This election is the single, explicit choice for the project. The AGPL-3.0 arm
is hereby NOT exercised.

## Usage / redistribution warning

Per the MPL-2.0 election above, redistributing built binaries that link this
code is permitted under MPL-2.0 terms: preserve this NOTICE, make the pinned
OpenVPN3 source available to recipients, and keep any future modifications to
the submodule's own files under MPL-2.0. lensr's first-party files retain their
own (non-MPL) license. The AGPL-3.0 network-copyleft path is NOT elected and its
obligations do not apply.
