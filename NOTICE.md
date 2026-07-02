# Third-Party Notices

This module VENDORS pinned, pruned copies of its C/C++ build dependencies so
that `go get github.com/inovacc/openvpn3-go` + a C toolchain builds the cgo
engine with no extra fetch step. Vendored trees are refreshed only by the
maintainer tool (`task vendor:sync`); their checksums are printed by
`task vendor:verify`.

## Vendored components

| Tree | Upstream | Pin | License | sha256 (dirhash, LF-normalized) |
|------|----------|-----|---------|----------------------------------|
| `openvpn/` | https://github.com/OpenVPN/openvpn3 | `5b7841a847619e9e1ba3f7371e0c9e2743383481` (tag `release/3.11.6`) | MPL-2.0 OR AGPL-3.0-only (dual; see election below) | `5fed8e5e743cff0feaabbbdea65eb9f181342f54c26e66e6a2c22fcd74833086` |
| `deps/asio/` | https://github.com/chriskohlhoff/asio | tag `asio-1-24-0` | Boost Software License 1.0 | `7ea6ae8980cc307fae608e8c24227b3641aa11de2670f1c71b1481d9a5e8be88` |
| `deps/lz4/` | https://github.com/lz4/lz4 | tag `v1.8.3` | BSD 2-Clause (lib/) | `dbf2894929545480b7b4cfa448212503818e7b6ab76a77590c215e73dcf65927` |
| `deps/jsoncpp/` | https://github.com/open-source-parsers/jsoncpp | tag `1.8.4` | MIT / Public Domain | `cb0c78304b342a5bc60416f7af31b7c7dc72fedf04052820093fca15389512b4` |
| `deps/tap-windows/` | https://github.com/OpenVPN/tap-windows6 | vendored at HEAD `0cad8664c2a51832df61f2e1853b6da317d1c129` on 2026-07-02 (upstream ref is HEAD/unpinned) | MIT (this header file only; the driver itself is GPLv2) | `ac3e15a3e71a778c463e11b542911044247075d9efbf41267906982df7b44128` |

Each tree retains its upstream license file(s) in place
(`openvpn/LICENSE.md`, `openvpn/LICENSES/`, `deps/asio/asio/LICENSE_1_0.txt`,
`deps/lz4/LICENSE`, `deps/jsoncpp/LICENSE`).

Local modification: `openvpn/openvpn/win/call.hpp` carries a two-line patch
(CREATE_NO_WINDOW / SW_HIDE) so netsh/route child processes spawn without a
visible window. Per the MPL-2.0 election below, this modified MPL file is
available under MPL-2.0 in this repository.

## OpenVPN3 license election

The pinned OpenVPN3 core is dual-licensed **AGPL-3.0-only OR MPL-2.0** (with an
OpenSSL linking exception on the AGPL arm). This module ELECTS the **MPL-2.0**
arm for its use, distribution, and linking of the OpenVPN3 core (including the
cgo binding and the C++ shim under `shim/` that links against it).

What electing MPL-2.0 means:

- **File-level (weak) copyleft.** MPL-2.0 attaches per file. Only
  modifications to MPL-2.0-licensed source files (currently: the `call.hpp`
  patch above) must be made available under MPL-2.0 — which this repository
  does by carrying them in the vendored tree. First-party Go/C/C++ files are
  NOT MPL-2.0-licensed merely by linking.
- **No network copyleft.** The AGPL-3.0 §13 network/SaaS source-disclosure
  obligation is NOT triggered; that arm is not exercised.
- **Source availability.** Distributing binaries that link this code requires
  making the MPL-2.0 source available — satisfied by this repository itself,
  which contains the complete vendored tree at the pin above.

This election is the single, explicit choice for the module. Downstream
consumers make their own election but inherit this repository's MPL-2.0
compliance posture by default.
