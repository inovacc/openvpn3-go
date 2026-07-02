# Reusable Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `github.com/inovacc/openvpn3-go` consumable via plain `go get` — vendored pinned C/C++ sources, env-var OpenSSL, bootstrap reduced to a maintainer tool.

**Architecture:** Commit pruned copies of the OpenVPN3 core + asio/lz4/jsoncpp/tap-windows trees at their current `${SRCDIR}`-relative paths (so `cgo_flags.go` is untouched), delete the consumer-facing `bootstrap/` package, and add a maintainer-only `tools/` nested module (`vendorsync` refreshes vendored trees; `modverify` builds the module zip and a local `file://` proxy to prove read-only module-cache builds work).

**Tech Stack:** Go 1.25.0, cgo (MinGW/gcc on Windows), `golang.org/x/mod` (tools module only), git, Taskfile.

**Spec:** `docs/superpowers/specs/2026-07-02-reusable-module-design.md`

## Global Constraints

- Pinned upstreams (never change silently): OpenVPN3 core SHA `5b7841a847619e9e1ba3f7371e0c9e2743383481` (tag `release/3.11.6`), asio tag `asio-1-24-0`, lz4 tag `v1.8.3`, jsoncpp tag `1.8.4`, tap-windows6 (HEAD, record SHA when vendoring).
- `cgo_flags.go` `#cgo` directives must NOT change — vendored trees land at the exact paths those flags already reference.
- `CGO_ENABLED=0 go build ./...` and `CGO_ENABLED=0 go test -short ./...` must pass after every task.
- The main module's only Go dependency stays `golang.org/x/sys` + `golang.org/x/term` (check `go.mod`); `golang.org/x/mod` goes in the **nested** `tools/go.mod` only.
- Conventional commits; NO AI attribution lines in commit messages.
- Errors: lowercase, wrapped with `%w`, matched with `errors.Is`.
- On this machine the working OpenSSL location is `C:\Users\dyamm\scoop\apps\mingw\current\opt` (headers `include/`, libs `lib/`); `cgo_openssl_local.go` (gitignored) already points at it.
- Full cgo rebuilds take ~2–5 minutes (single-TU OpenVPN3 core); don't interpret slowness as a hang.
- All shell snippets below are Git Bash syntax; repo root is `D:\weaver-sync\development\personal\openvpn3-go` (`/d/weaver-sync/development/personal/openvpn3-go`).

---

### Task 1: Commit the pending test fixture

`testdata/sample.ovpn` was created earlier this session (fixes `TestConfigFromOVPN_KeepsInlineContent`, whose fixture was never committed with the initial import) but is still uncommitted.

**Files:**
- Commit (already on disk): `testdata/sample.ovpn`

**Interfaces:**
- Consumes: nothing
- Produces: green `go test -short ./...` baseline for all later tasks

- [ ] **Step 1: Verify the fixture exists and tests pass**

Run: `CGO_ENABLED=0 go test -short ./...`
Expected: `ok  github.com/inovacc/openvpn3-go` (no FAIL lines)

- [ ] **Step 2: Commit**

```bash
git add testdata/sample.ovpn
git commit -m "test: add missing sample.ovpn fixture for profile tests"
```

---

### Task 2: Vendor the pruned C/C++ trees

The bootstrap-fetched trees already sit at the right paths (`openvpn/`, `deps/`) — currently gitignored full clones. Prune them to what the build includes, un-ignore, prove the build still works, commit (~12.6 MB, ~1,250 files).

**Files:**
- Modify: `.gitignore` (remove lines 10–12)
- Commit (pruned trees): `openvpn/`, `deps/asio/`, `deps/lz4/`, `deps/jsoncpp/`, `deps/tap-windows/`

**Interfaces:**
- Consumes: bootstrap-fetched trees on disk (present from this session's bootstrap run)
- Produces: committed vendored trees at the exact paths `cgo_flags.go` references; later tasks rely on `openvpn/openvpn/win/call.hpp` containing the CREATE_NO_WINDOW patch (already applied by this session's bootstrap run — verify in Step 2)

- [ ] **Step 1: Prune each tree to its keep-list**

```bash
cd /d/weaver-sync/development/personal/openvpn3-go

# OpenVPN3 core: keep client/, openvpn/, LICENSE.md, LICENSES/
(cd openvpn && find . -mindepth 1 -maxdepth 1 \
  ! -name client ! -name openvpn ! -name LICENSE.md ! -name LICENSES \
  -exec rm -rf {} +)

# asio: keep asio/include/ + asio/LICENSE_1_0.txt (nested asio/ layout stays)
(cd deps/asio && find . -mindepth 1 -maxdepth 1 ! -name asio -exec rm -rf {} +)
(cd deps/asio/asio && find . -mindepth 1 -maxdepth 1 \
  ! -name include ! -name LICENSE_1_0.txt -exec rm -rf {} +)

# lz4: keep lib/ + LICENSE (cgo_lz4.c includes deps/lz4/lib/lz4.c — sources needed)
(cd deps/lz4 && find . -mindepth 1 -maxdepth 1 ! -name lib ! -name LICENSE -exec rm -rf {} +)

# jsoncpp: keep include/ + LICENSE + AUTHORS. src/ is NOT vendored:
# HAVE_JSONCPP is never defined in cgo_flags.go, so no jsoncpp symbols are
# compiled or linked (today's link already succeeds with zero jsoncpp objects).
(cd deps/jsoncpp && find . -mindepth 1 -maxdepth 1 \
  ! -name include ! -name LICENSE ! -name AUTHORS -exec rm -rf {} +)

# deps/tap-windows/ is already minimal (include/tap-windows.h only)

# Strip all nested git metadata and ignore files (a nested .gitignore would
# hide vendored files from `git add`)
find openvpn deps -name '.git' -prune -exec rm -rf {} + 2>/dev/null
find openvpn deps \( -name '.gitignore' -o -name '.gitattributes' -o -name '.github' \) -exec rm -rf {} + 2>/dev/null
du -sh openvpn deps
```

Expected: `openvpn` ≈ 5 MB, `deps` ≈ 7.5 MB.

- [ ] **Step 2: Verify the call.hpp patch is present in the vendored copy**

Run: `grep -c 'CREATE_NO_WINDOW' openvpn/openvpn/win/call.hpp`
Expected: `1` or more. If `0`, the patch is missing — apply it by running `go run ./cmd/openvpn bootstrap` once BEFORE Task 5 removes the fetch logic, then re-run this grep.

- [ ] **Step 3: Un-ignore the vendored paths**

In `.gitignore`, delete exactly these three lines (keep the `cgo_openssl_local.go` lines):

```
# Bootstrap-fetched C/C++ dependencies (re-created by cmd/openvpn).
/deps/
/openvpn/
```

- [ ] **Step 4: Prove the pruned set builds (cold cache)**

```bash
go clean -cache
CGO_ENABLED=1 go build ./...
CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test -short ./...
```

Expected: both builds succeed (cgo build takes minutes; the only compiler output should be the known benign `-Wsubobject-linkage` warning from `shim/ovpncli_shim.cpp:178`), tests `ok`. If the cgo build fails with a missing header, the prune cut too deep — identify the header path from the error, restore that subtree by re-running `go run ./cmd/openvpn bootstrap --force`, re-prune with the missing entry added to the keep-list above, and update this plan's keep-list comment.

- [ ] **Step 5: Commit the vendored trees**

```bash
git add .gitignore openvpn deps
git status --short | wc -l   # sanity: expect roughly 1200-1400 files
git commit -m "feat: vendor pinned OpenVPN3 core and C/C++ deps for go-get consumption"
```

---

### Task 3: `tools/` nested module with `vendorsync`

Maintainer tool that re-fetches the pinned upstreams, prunes to the keep-lists, applies the call.hpp patch, and prints a provenance/checksum table. Nested module so its dependency (`golang.org/x/mod`, used by Task 4's `modverify`) never touches the main `go.mod`, and so `tools/` is automatically excluded from the module zip.

**Files:**
- Create: `tools/go.mod`
- Create: `tools/vendorsync/main.go`
- Create: `tools/vendorsync/dirhash.go`
- Test: `tools/vendorsync/dirhash_test.go`
- Modify: `Taskfile.yml` (append tasks)

**Interfaces:**
- Consumes: nothing from other tasks (ports code from `bootstrap/bootstrap.go` + `bootstrap/bootstrap_windows.go`, which still exist until Task 5)
- Produces: `dirHash(dir string) (string, error)` (in package main, shared by files); CLI `go run ./vendorsync [-root DIR] [-verify]`; Taskfile targets `vendor:sync`, `vendor:verify`, `tools:check`. Task 6 pastes the `-verify` table into NOTICE.md.

- [ ] **Step 1: Create the nested module**

```bash
mkdir -p tools/vendorsync
cd tools && go mod init github.com/inovacc/openvpn3-go/tools && cd ..
```

- [ ] **Step 2: Write the failing dirhash test**

Create `tools/vendorsync/dirhash_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirHash(t *testing.T) {
	write := func(dir string, files map[string]string) string {
		t.Helper()
		for name, content := range files {
			p := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}

	tests := []struct {
		name   string
		a, b   map[string]string
		equal  bool
	}{
		{
			name:  "identical trees hash equal",
			a:     map[string]string{"x/a.h": "alpha", "b.h": "beta"},
			b:     map[string]string{"x/a.h": "alpha", "b.h": "beta"},
			equal: true,
		},
		{
			name:  "content change hashes differ",
			a:     map[string]string{"a.h": "alpha"},
			b:     map[string]string{"a.h": "ALPHA"},
			equal: false,
		},
		{
			name:  "path change hashes differ",
			a:     map[string]string{"a.h": "alpha"},
			b:     map[string]string{"c.h": "alpha"},
			equal: false,
		},
		{
			name:  "crlf normalizes to lf",
			a:     map[string]string{"a.h": "line1\r\nline2\r\n"},
			b:     map[string]string{"a.h": "line1\nline2\n"},
			equal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ha, err := dirHash(write(t.TempDir(), tt.a))
			if err != nil {
				t.Fatal(err)
			}
			hb, err := dirHash(write(t.TempDir(), tt.b))
			if err != nil {
				t.Fatal(err)
			}
			if (ha == hb) != tt.equal {
				t.Fatalf("equal=%v want %v (a=%s b=%s)", ha == hb, tt.equal, ha, hb)
			}
		})
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go -C tools test ./vendorsync/`
Expected: FAIL — `undefined: dirHash`

- [ ] **Step 4: Implement dirhash**

Create `tools/vendorsync/dirhash.go`:

```go
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// dirHash returns a deterministic sha256 over a tree: sorted relative paths
// (slash-separated) + LF-normalized contents. CRLF is normalized so the hash
// is stable across git autocrlf settings on different machines.
func dirHash(dir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk %s: %w", dir, err)
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, rel := range paths {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", rel, err)
		}
		fmt.Fprintf(h, "%s\n", rel)
		h.Write(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go -C tools test ./vendorsync/`
Expected: PASS

- [ ] **Step 6: Write the vendorsync main**

Create `tools/vendorsync/main.go` (ports fetch/patch logic from `bootstrap/bootstrap.go` and `bootstrap/bootstrap_windows.go`):

```go
// Command vendorsync is the MAINTAINER tool that refreshes the vendored C/C++
// trees in the module root: it re-fetches each pinned upstream, copies only the
// keep-list into place, applies the call.hpp patch, strips git metadata, and
// prints a provenance/checksum table for NOTICE.md.
//
// Consumers never run this — the vendored trees ship in the module.
//
// Usage (from repo root):
//
//	go -C tools run ./vendorsync -verify        # hash current trees only
//	go -C tools run ./vendorsync                # re-fetch + replace all trees
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// tree is one vendored upstream: clone URL, pinned ref, and a keep-map of
// clone-relative source path -> module-root-relative destination path.
type tree struct {
	Name string
	URL  string
	Ref  string // full commit SHA or tag; "" = default branch HEAD
	Keep map[string]string
}

var trees = []tree{
	{
		Name: "openvpn3",
		URL:  "https://github.com/OpenVPN/openvpn3",
		// release/3.11.6 — newer commits need libfmt, which we do not vendor.
		Ref: "5b7841a847619e9e1ba3f7371e0c9e2743383481",
		Keep: map[string]string{
			"client":     "openvpn/client",
			"openvpn":    "openvpn/openvpn",
			"LICENSE.md": "openvpn/LICENSE.md",
			"LICENSES":   "openvpn/LICENSES",
		},
	},
	{
		Name: "asio",
		URL:  "https://github.com/chriskohlhoff/asio.git",
		Ref:  "asio-1-24-0",
		Keep: map[string]string{
			"asio/include":         "deps/asio/asio/include",
			"asio/LICENSE_1_0.txt": "deps/asio/asio/LICENSE_1_0.txt",
		},
	},
	{
		Name: "lz4",
		URL:  "https://github.com/lz4/lz4.git",
		Ref:  "v1.8.3",
		Keep: map[string]string{
			"lib":     "deps/lz4/lib",
			"LICENSE": "deps/lz4/LICENSE",
		},
	},
	{
		Name: "jsoncpp",
		URL:  "https://github.com/open-source-parsers/jsoncpp.git",
		Ref:  "1.8.4",
		// include/ only: HAVE_JSONCPP is never defined, no jsoncpp TU is linked.
		Keep: map[string]string{
			"include": "deps/jsoncpp/include",
			"LICENSE": "deps/jsoncpp/LICENSE",
			"AUTHORS": "deps/jsoncpp/AUTHORS",
		},
	},
	{
		Name: "tap-windows6",
		URL:  "https://github.com/OpenVPN/tap-windows6.git",
		Ref:  "", // HEAD; the resolved SHA is printed for the NOTICE table
		Keep: map[string]string{
			"src/tap-windows.h": "deps/tap-windows/include/tap-windows.h",
		},
	},
}

// vendorRoots are the module-root-relative tree roots hashed by -verify, in
// NOTICE.md table order.
var vendorRoots = []struct{ name, path string }{
	{"openvpn3", "openvpn"},
	{"asio", "deps/asio"},
	{"lz4", "deps/lz4"},
	{"jsoncpp", "deps/jsoncpp"},
	{"tap-windows6", "deps/tap-windows"},
}

func main() {
	root := flag.String("root", "", "module root (default: git toplevel)")
	verify := flag.Bool("verify", false, "hash current vendored trees; do not fetch")
	flag.Parse()

	r, err := resolveRoot(*root)
	if err != nil {
		fatal(err)
	}

	if !*verify {
		for _, t := range trees {
			if err := syncTree(r, t); err != nil {
				fatal(fmt.Errorf("sync %s: %w", t.Name, err))
			}
		}
		if err := patchCallHpp(r); err != nil {
			fatal(fmt.Errorf("patch call.hpp: %w", err))
		}
	}
	printTable(r)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "vendorsync:", err)
	os.Exit(1)
}

// resolveRoot returns the explicit root or the enclosing git toplevel, and
// validates it is the openvpn3-go module root.
func resolveRoot(explicit string) (string, error) {
	r := explicit
	if r == "" {
		out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return "", fmt.Errorf("resolve git toplevel: %w", err)
		}
		r = strings.TrimSpace(string(out))
	}
	mod, err := os.ReadFile(filepath.Join(r, "go.mod"))
	if err != nil || !strings.Contains(string(mod), "module github.com/inovacc/openvpn3-go") {
		return "", fmt.Errorf("%s is not the openvpn3-go module root", r)
	}
	return r, nil
}

// syncTree clones t at its pinned ref into a temp dir and replaces each
// keep-map destination under root with the freshly fetched copy.
func syncTree(root string, t tree) error {
	tmp, err := os.MkdirTemp("", "vendorsync-"+t.Name)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	fmt.Fprintf(os.Stderr, "[fetch] %s @ %s\n", t.Name, refLabel(t.Ref))
	if isSHA(t.Ref) {
		for _, args := range [][]string{
			{"init", "-q"},
			{"remote", "add", "origin", t.URL},
			{"-c", "core.autocrlf=false", "fetch", "--depth", "1", "origin", t.Ref},
			{"-c", "core.autocrlf=false", "checkout", "-q", "FETCH_HEAD"},
		} {
			if err := run(tmp, "git", args...); err != nil {
				return err
			}
		}
	} else {
		args := []string{"-c", "core.autocrlf=false", "clone", "--depth", "1"}
		if t.Ref != "" {
			args = append(args, "--branch", t.Ref)
		}
		args = append(args, t.URL, tmp)
		if err := run("", "git", args...); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "[pin]   %s HEAD = %s\n", t.Name, headSHA(tmp))

	for src, dst := range t.Keep {
		from := filepath.Join(tmp, filepath.FromSlash(src))
		to := filepath.Join(root, filepath.FromSlash(dst))
		if err := os.RemoveAll(to); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := copyTree(from, to); err != nil {
			return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
		}
	}
	return nil
}

func refLabel(ref string) string {
	if ref == "" {
		return "HEAD"
	}
	return ref
}

func isSHA(ref string) bool {
	if len(ref) != 40 {
		return false
	}
	for _, c := range ref {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// copyTree copies a file or directory, skipping git metadata files.
func copyTree(from, to string) error {
	fi, err := os.Stat(from)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		b, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, 0o644)
	}
	return filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if name == ".git" || name == ".gitignore" || name == ".gitattributes" || name == ".github" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
}

var creationFlagsRE = regexp.MustCompile(`(?m)^(\s*)0,(\s*)// creation flags`)

// patchCallHpp makes the vendored OpenVPN3 command runner spawn child processes
// (netsh/route/ipconfig) with no visible window. Idempotent.
func patchCallHpp(root string) error {
	p := filepath.Join(root, "openvpn", "openvpn", "win", "call.hpp")
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	src := string(b)
	orig := src

	if !strings.Contains(src, "CREATE_NO_WINDOW") {
		src = creationFlagsRE.ReplaceAllString(src, "${1}CREATE_NO_WINDOW,${2}// creation flags (openvpn3-go: hide netsh/route child windows)")
	}
	if !strings.Contains(src, "STARTF_USESHOWWINDOW") {
		src = strings.Replace(src,
			"siStartInfo.dwFlags |= STARTF_USESTDHANDLES;",
			"siStartInfo.dwFlags |= STARTF_USESTDHANDLES | STARTF_USESHOWWINDOW;\n    siStartInfo.wShowWindow = SW_HIDE; // openvpn3-go: hide netsh/route child windows",
			1)
	}
	if src == orig {
		return nil
	}
	fmt.Fprintln(os.Stderr, "[patch] call.hpp (hide child windows)")
	return os.WriteFile(p, []byte(src), 0o644)
}

// printTable emits the NOTICE.md provenance table rows (name | checksum).
func printTable(root string) {
	fmt.Println("| tree | sha256 (dirhash, LF-normalized) |")
	fmt.Println("|------|----------------------------------|")
	for _, v := range vendorRoots {
		h, err := dirHash(filepath.Join(root, filepath.FromSlash(v.path)))
		if err != nil {
			fatal(err)
		}
		fmt.Printf("| %s (`%s/`) | `%s` |\n", v.name, v.path, h)
	}
}

func headSHA(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}
```

- [ ] **Step 7: Build and run -verify**

```bash
go -C tools vet ./... && go -C tools test ./...
go -C tools run ./vendorsync -verify
```

Expected: vet/test clean; a 5-row markdown table with sha256 values printed. Save this output — Task 6 pastes it into NOTICE.md.

- [ ] **Step 8: Add Taskfile targets**

Append to `Taskfile.yml` under `tasks:`:

```yaml
  vendor:sync:
    desc: "MAINTAINER: re-fetch pinned upstreams and refresh vendored C/C++ trees"
    cmds:
      - go -C tools run ./vendorsync

  vendor:verify:
    desc: Print provenance checksums of the vendored C/C++ trees
    cmds:
      - go -C tools run ./vendorsync -verify

  tools:check:
    desc: Vet and test the tools module
    cmds:
      - go -C tools vet ./...
      - go -C tools test ./...
```

Run: `task vendor:verify`
Expected: same table as Step 7.

- [ ] **Step 9: Commit**

```bash
git add tools Taskfile.yml
git commit -m "feat: add maintainer vendorsync tool in nested tools module"
```

---

### Task 4: `modverify` — module zip check + local file:// proxy

Proves the module is consumable read-only: builds the module zip exactly as a proxy would serve it (from `git archive`, so untracked files like `cgo_openssl_local.go` are excluded), which validates size/case-collision/path rules, and emits a local `file://` proxy layout Task 7's scratch consumer uses.

**Files:**
- Create: `tools/modverify/main.go`
- Modify: `tools/go.mod` (adds `golang.org/x/mod`)
- Modify: `Taskfile.yml` (append task)

**Interfaces:**
- Consumes: `tools/go.mod` from Task 3
- Produces: CLI `go -C tools run ./modverify -version vX.Y.Z [-proxy DIR]`; proxy layout `<DIR>/github.com/inovacc/openvpn3-go/@v/{list,vX.Y.Z.info,vX.Y.Z.mod,vX.Y.Z.zip}`; Taskfile target `module:verify`. Task 7 consumes the proxy dir.

- [ ] **Step 1: Add the x/mod dependency**

```bash
cd tools && go get golang.org/x/mod@latest && go mod tidy && cd ..
```

- [ ] **Step 2: Write modverify**

Create `tools/modverify/main.go`:

```go
// Command modverify proves the module publishes and consumes cleanly WITHOUT a
// writable checkout:
//
//  1. exports HEAD via `git archive` (exactly the file set a proxy would zip —
//     untracked files like cgo_openssl_local.go are excluded),
//  2. builds the module zip with golang.org/x/mod/zip.CreateFromDir, which
//     enforces proxy rules (500 MiB limit, case-insensitive collisions,
//     nested-module exclusion),
//  3. optionally lays the zip out as a file:// GOPROXY for a scratch consumer.
//
// Usage (from repo root):
//
//	go -C tools run ./modverify -version v0.1.1 -proxy ../dist/goproxy
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/zip"
)

const modPath = "github.com/inovacc/openvpn3-go"

func main() {
	version := flag.String("version", "", "semver to stamp, e.g. v0.1.1 (required)")
	proxy := flag.String("proxy", "", "emit a file:// GOPROXY layout into this dir")
	flag.Parse()
	if *version == "" || !strings.HasPrefix(*version, "v") {
		fatal(fmt.Errorf("-version vX.Y.Z is required"))
	}

	root, err := gitToplevel()
	if err != nil {
		fatal(err)
	}

	// 1. Export HEAD to a temp dir via git archive.
	export, err := os.MkdirTemp("", "modverify-export")
	if err != nil {
		fatal(err)
	}
	defer func() { _ = os.RemoveAll(export) }()
	archive := exec.Command("git", "-C", root, "archive", "--format=tar", "HEAD")
	untar := exec.Command("tar", "-x", "-C", export)
	untar.Stdin, _ = archive.StdoutPipe()
	untar.Stderr = os.Stderr
	if err := untar.Start(); err != nil {
		fatal(err)
	}
	if err := archive.Run(); err != nil {
		fatal(fmt.Errorf("git archive: %w", err))
	}
	if err := untar.Wait(); err != nil {
		fatal(fmt.Errorf("untar archive: %w", err))
	}

	// 2. Build + validate the module zip.
	mv := module.Version{Path: modPath, Version: *version}
	zipPath := filepath.Join(os.TempDir(), "openvpn3-go-"+*version+".zip")
	f, err := os.Create(zipPath)
	if err != nil {
		fatal(err)
	}
	if err := zip.CreateFromDir(f, mv, export); err != nil {
		fatal(fmt.Errorf("module zip validation FAILED: %w", err))
	}
	if err := f.Close(); err != nil {
		fatal(err)
	}
	fi, _ := os.Stat(zipPath)
	fmt.Printf("module zip OK: %s (%.1f MB)\n", zipPath, float64(fi.Size())/1e6)

	// 3. Optional file:// proxy layout.
	if *proxy == "" {
		return
	}
	vdir := filepath.Join(*proxy, filepath.FromSlash(modPath), "@v")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		fatal(err)
	}
	gomod, err := os.ReadFile(filepath.Join(export, "go.mod"))
	if err != nil {
		fatal(err)
	}
	info, _ := json.Marshal(map[string]string{"Version": *version, "Time": "2026-07-02T00:00:00Z"})
	zipBytes, err := os.ReadFile(zipPath)
	if err != nil {
		fatal(err)
	}
	for name, content := range map[string][]byte{
		"list":              []byte(*version + "\n"),
		*version + ".info":  info,
		*version + ".mod":   gomod,
		*version + ".zip":   zipBytes,
	} {
		if err := os.WriteFile(filepath.Join(vdir, name), content, 0o644); err != nil {
			fatal(err)
		}
	}
	abs, _ := filepath.Abs(*proxy)
	fmt.Printf("proxy ready: GOPROXY=file:///%s\n", filepath.ToSlash(abs))
}

func gitToplevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolve git toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "modverify:", err)
	os.Exit(1)
}
```

Note: `module.EscapePath` is unnecessary here — `github.com/inovacc/openvpn3-go` contains no uppercase letters, so its escaped path equals itself. If the module path ever gains uppercase, switch `filepath.FromSlash(modPath)` to use `module.EscapePath(modPath)` first.

- [ ] **Step 3: Run it (zip validation only)**

Run: `go -C tools vet ./... && go -C tools run ./modverify -version v0.1.1`
Expected: `module zip OK: ... (~13-15 MB)`. A failure here means the vendored trees violate proxy rules (e.g. case-insensitive collision) — the error names the offending files; rename/remove them in the vendored tree, fix the keep-lists in `tools/vendorsync/main.go` to match, and re-run.

- [ ] **Step 4: Add Taskfile target**

Append to `Taskfile.yml` under `tasks:`:

```yaml
  module:verify:
    desc: Validate the module zip and build a local file:// GOPROXY under dist/goproxy
    cmds:
      - go -C tools run ./modverify -version v0.1.1 -proxy ../dist/goproxy
```

(`dist/` is already gitignored.)

Run: `task module:verify`
Expected: `module zip OK` + `proxy ready: GOPROXY=file:///...`

- [ ] **Step 5: Commit**

```bash
git add tools Taskfile.yml
git commit -m "feat: add modverify module-zip check and local proxy builder"
```

---

### Task 5: Delete `bootstrap/`, shrink the CLI subcommand to an OpenSSL dev helper

Consumers no longer fetch anything. The `openvpn bootstrap` subcommand keeps its name but now only detects OpenSSL and writes the gitignored `cgo_openssl_local.go` (maintainer/dev convenience). `RuntimeDLLSourceDir` is dropped (referenced by nothing — verified in Step 1).

**Files:**
- Delete: `bootstrap/` (all five files)
- Create: `cmd/openvpn/openssl_windows.go`
- Create: `cmd/openvpn/openssl_other.go`
- Modify: `cmd/openvpn/main.go` (imports, `cmdBootstrap`, usage text)

**Interfaces:**
- Consumes: vendorsync (Task 3) must already be committed — it replaced bootstrap's fetch role.
- Produces: `setupOpenSSL(root string) error` (package main, per-OS files); `openvpn bootstrap [--root DIR]` CLI behavior. Public package `github.com/inovacc/openvpn3-go/bootstrap` ceases to exist (approved v0.1.0 breakage per spec).

- [ ] **Step 1: Verify nothing else references the bootstrap package**

Run: `grep -rn 'openvpn3-go/bootstrap\|RuntimeDLLSourceDir\|bootstrap\.' --include='*.go' --include='*.yml' --include='*.yaml' . | grep -v '^./bootstrap/' | grep -v tools/ | grep -v docs/`
Expected: only `cmd/openvpn/main.go` lines (the import and `bootstrap.Run` / `bootstrap.Options` in `cmdBootstrap`). If `.goreleaser` or a workflow references `RuntimeDLLSourceDir` or the bootstrap package, STOP and surface it before deleting.

- [ ] **Step 2: Create the per-OS OpenSSL helper**

Create `cmd/openvpn/openssl_windows.go` (ported from `bootstrap/bootstrap_windows.go`, with the scoop-mingw `opt` layout added to the candidates — the location this machine actually uses):

```go
//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// setupOpenSSL finds an OpenSSL dev install and writes the gitignored
// cgo_openssl_local.go in the module root so local cgo builds need no env.
// Maintainer/dev convenience only — consumers set CGO_CPPFLAGS/CGO_LDFLAGS
// instead (see README "Linking OpenSSL").
func setupOpenSSL(root string) error {
	ssl := resolveOpenSSL()
	if ssl == "" {
		return fmt.Errorf("no OpenSSL dev install found; set OPENVPN3_SSLDIR to a dir containing include/openssl and lib, or set CGO_CPPFLAGS/CGO_LDFLAGS manually")
	}
	fmt.Fprintf(os.Stderr, "openvpn: OpenSSL at %s\n", ssl)

	fwd := strings.ReplaceAll(ssl, `\`, "/")
	content := fmt.Sprintf(`//go:build windows && cgo

// Code generated by "openvpn bootstrap". DO NOT EDIT.
// Machine-specific OpenSSL paths; gitignored.
package openvpn3

// #cgo CPPFLAGS: -I%s/include
// #cgo LDFLAGS: -L%s/lib
import "C"
`, fwd, fwd)

	out := filepath.Join(root, "cgo_openssl_local.go")
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "openvpn: generated %s\n", out)
	return nil
}

// resolveOpenSSL finds a root that ships OpenSSL dev headers. Order:
// OPENVPN3_SSLDIR, LENSR_UCRT64 (legacy), scoop msys2 ucrt64, scoop mingw opt,
// C:\msys64\ucrt64. Empty if none validate.
func resolveOpenSSL() string {
	var candidates []string
	for _, env := range []string{"OPENVPN3_SSLDIR", "LENSR_UCRT64"} {
		if v := os.Getenv(env); v != "" {
			candidates = append(candidates, v)
		}
	}
	if up := os.Getenv("USERPROFILE"); up != "" {
		candidates = append(candidates,
			filepath.Join(up, "scoop", "apps", "msys2", "current", "ucrt64"),
			filepath.Join(up, "scoop", "apps", "mingw", "current", "opt"),
		)
	}
	candidates = append(candidates, `C:\msys64\ucrt64`)

	for _, c := range candidates {
		if fi, err := os.Stat(filepath.Join(c, "include", "openssl", "opensslv.h")); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}
```

Create `cmd/openvpn/openssl_other.go`:

```go
//go:build !windows

package main

import (
	"fmt"
	"os"
)

// setupOpenSSL is a no-op off Windows: unix/darwin toolchains find OpenSSL on
// their default search paths (or via CGO_CPPFLAGS/CGO_LDFLAGS).
func setupOpenSSL(root string) error {
	fmt.Fprintln(os.Stderr, "openvpn: nothing to do on this OS (OpenSSL comes from default toolchain paths)")
	return nil
}
```

- [ ] **Step 3: Rewire cmd/openvpn/main.go**

In `cmd/openvpn/main.go`:

1. Delete the import line `"github.com/inovacc/openvpn3-go/bootstrap"`.
2. Replace the whole `cmdBootstrap` function with:

```go
func cmdBootstrap(args []string) int {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	root := fs.String("root", ".", "module checkout root (dir containing go.mod)")
	_ = fs.Parse(args)

	mod, err := os.ReadFile(filepath.Join(*root, "go.mod"))
	if err != nil || !strings.Contains(string(mod), "module github.com/inovacc/openvpn3-go") {
		fmt.Fprintf(os.Stderr, "openvpn bootstrap: %s is not the openvpn3-go checkout root (run from a source checkout, or pass --root)\n", *root)
		return 1
	}
	if err := setupOpenSSL(*root); err != nil {
		fmt.Fprintln(os.Stderr, "openvpn bootstrap: error:", err)
		return 1
	}
	fmt.Fprintln(os.Stderr, "openvpn bootstrap: done")
	return 0
}
```

3. Add `"path/filepath"` to the imports (needed by the new `cmdBootstrap`).
4. In `usage()`, replace the line
   `openvpn bootstrap [--force]              fetch/init the C/C++ build dependencies`
   with
   `openvpn bootstrap [--root DIR]           dev helper: write machine-local OpenSSL cgo flags`
5. Update the package doc comment: replace the sentence about bootstrapping C/C++ build dependencies with "C/C++ dependencies are vendored in the module; `bootstrap` is a dev helper that writes machine-local OpenSSL cgo flags."

- [ ] **Step 4: Delete the bootstrap package**

```bash
git rm -r bootstrap
```

- [ ] **Step 5: Verify both build modes and the helper**

```bash
CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test -short ./...
CGO_ENABLED=1 go build ./...
CGO_ENABLED=0 go run ./cmd/openvpn bootstrap
```

Expected: builds pass; the helper prints `OpenSSL at C:\Users\dyamm\scoop\apps\mingw\current\opt` (or the msys2 path) and regenerates `cgo_openssl_local.go`.

- [ ] **Step 6: Commit**

```bash
git add cmd/openvpn
git commit -m "refactor!: drop bootstrap package; deps are vendored, CLI keeps OpenSSL dev helper"
```

---

### Task 6: Documentation reconciliation

**Files:**
- Modify: `NOTICE.md` (rewrite)
- Modify: `SIGNATURES.md` (header block only)
- Modify: `README.md` (add consumption + OpenSSL sections; bump `<!-- rev:NNN -->` if present, add `<!-- rev:001 -->` after the H1 if absent)
- Modify: `AGENTS.md` (project overview + dev tips; bump rev tag if present)
- Modify: `docs/ROADMAP.md`, `docs/INTEGRATION.md` (reconcile; bump `rev:001` → `rev:002` in INTEGRATION.md)
- Modify: `doc.go` (only if it mentions bootstrap — check with grep)

**Interfaces:**
- Consumes: the checksum table printed by `task vendor:verify` (Task 3 Step 7) and the tap-windows6 SHA printed by vendorsync (or `git -C <clone> rev-parse HEAD` noted at vendor time; if unknown, run `task vendor:sync` once and record the `[pin] tap-windows6 HEAD = ...` line).
- Produces: docs matching the vendored model; nothing later depends on this task.

- [ ] **Step 1: Rewrite NOTICE.md**

Replace the entire file content with the following, then replace each `<sha256-from-vendor-verify>` with the real value from the `task vendor:verify` table and `<tap-windows6-sha>` with the recorded clone SHA. For the tap-windows.h license cell: run `head -40 deps/tap-windows/include/tap-windows.h` and transcribe the license the header itself declares (do not guess it).

```markdown
# Third-Party Notices

This module VENDORS pinned, pruned copies of its C/C++ build dependencies so
that `go get github.com/inovacc/openvpn3-go` + a C toolchain builds the cgo
engine with no extra fetch step. Vendored trees are refreshed only by the
maintainer tool (`task vendor:sync`); their checksums are printed by
`task vendor:verify`.

## Vendored components

| Tree | Upstream | Pin | License | sha256 (dirhash, LF-normalized) |
|------|----------|-----|---------|----------------------------------|
| `openvpn/` | https://github.com/OpenVPN/openvpn3 | `5b7841a847619e9e1ba3f7371e0c9e2743383481` (tag `release/3.11.6`) | MPL-2.0 OR AGPL-3.0-only (dual; see election below) | `<sha256-from-vendor-verify>` |
| `deps/asio/` | https://github.com/chriskohlhoff/asio | tag `asio-1-24-0` | Boost Software License 1.0 | `<sha256-from-vendor-verify>` |
| `deps/lz4/` | https://github.com/lz4/lz4 | tag `v1.8.3` | BSD 2-Clause (lib/) | `<sha256-from-vendor-verify>` |
| `deps/jsoncpp/` | https://github.com/open-source-parsers/jsoncpp | tag `1.8.4` | MIT / Public Domain | `<sha256-from-vendor-verify>` |
| `deps/tap-windows/` | https://github.com/OpenVPN/tap-windows6 | `<tap-windows6-sha>` | <license declared in tap-windows.h header> | `<sha256-from-vendor-verify>` |

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
```

- [ ] **Step 2: Fix SIGNATURES.md header block**

In `SIGNATURES.md`, update only the intro (keep the entire signatures body):
- Replace `Pinned submodule tag:` with `Pinned vendored tag:` and `Submodule commit:` with `Vendored commit:`.
- Replace `All line refs are relative to the submodule root \`pkg/openvpn3/openvpn/\`.` with `All line refs are relative to the vendored tree root \`openvpn/\`.`

- [ ] **Step 3: Add README consumption + OpenSSL sections**

Add to `README.md` (after the existing intro/features material), and maintain the rev tag per the file's state:

```markdown
## Using the module

```bash
go get github.com/inovacc/openvpn3-go
```

Pure-Go (stub engine, no C toolchain): build with `CGO_ENABLED=0`. The API is
identical; `openvpn3.Available()` reports `false` and connects return
`ErrUnavailable`.

Real engine (Windows): build with `CGO_ENABLED=1` and a MinGW-w64 gcc on PATH.
All C/C++ sources (OpenVPN3 core, asio, lz4, jsoncpp, tap-windows.h) are
vendored in the module — no bootstrap step, no `replace` directive, no writable
checkout needed. Only OpenSSL comes from your toolchain (next section).

## Linking OpenSSL

The cgo engine links `-lssl -lcrypto`. If your gcc ships OpenSSL on its
default search paths (Linux distros, msys2 ucrt64), it just works. If you see:

```
fatal error: openssl/opensslv.h: No such file or directory
```

or at link time:

```
cannot find -lssl / cannot find -lcrypto
```

point cgo at an OpenSSL dev install once:

```bash
export CGO_CPPFLAGS="-I/path/to/ssl/include"
export CGO_LDFLAGS="-L/path/to/ssl/lib"
```

Developers working in a source checkout can instead run
`go run ./cmd/openvpn bootstrap`, which detects OpenSSL and writes a
gitignored `cgo_openssl_local.go` so no env is needed.
```

- [ ] **Step 4: Reconcile AGENTS.md, ROADMAP.md, INTEGRATION.md, doc.go**

- `AGENTS.md` "Project overview": replace the `bootstrap/` sentence with: "C/C++ sources (OpenVPN3 core pinned at `5b7841a`, asio, lz4, jsoncpp, tap-windows.h) are **vendored** under `openvpn/` and `deps/`; `tools/` (nested module) holds the maintainer `vendorsync`/`modverify` tools."
- `AGENTS.md` "Dev environment tips": replace "Run the per-OS bootstrap in `bootstrap/` before building the cgo paths" with "No bootstrap needed — C/C++ deps are vendored. On Windows, run `go run ./cmd/openvpn bootstrap` once if OpenSSL isn't on your toolchain's default paths (writes gitignored `cgo_openssl_local.go`)."
- `docs/ROADMAP.md`: mark Phase 1 `Land native C/C++ sources` done with note "(vendored at module root — supersedes the old pkg/ovpn3/native layout)"; mark Phase 2 `Wire cgo bridge` done; update `Current Status` to "Vendored-source reusable module; cgo engine builds via plain go get + C toolchain"; fix remaining `pkg/ovpn3` / `internal/vpnsvc` path references to the actual flat layout.
- `docs/INTEGRATION.md`: update the intro ("bring the set into this scaffold") to describe the vendored model; replace invariant 1's `pkg/ovpn3` reference with the actual root package; replace `client_stub.go`/`client_cgo.go` mentions with the real file names (`engine_stub.go`, `cgo_flags.go`); bump `<!-- rev:001 -->` to `<!-- rev:002 -->`.
- `doc.go`: run `grep -n bootstrap doc.go` — if it mentions the bootstrap fetch step, update the wording to the vendored model; if no mention, leave it.

- [ ] **Step 5: Verify docs build nothing is broken, commit**

```bash
CGO_ENABLED=0 go build ./...   # doc.go edits still compile
git add NOTICE.md SIGNATURES.md README.md AGENTS.md docs/ROADMAP.md docs/INTEGRATION.md doc.go
git commit -m "docs: reconcile to vendored-source reusable-module model"
```

---

### Task 7: End-to-end verification

**Files:**
- No source changes expected. Read: `.github/workflows/build.yml`, `.github/workflows/test.yml`, `.github/workflows/release.yaml` (modify ONLY if a Windows job runs cgo — see Step 4).

**Interfaces:**
- Consumes: the proxy dir from `task module:verify` (Task 4); the committed vendored trees.
- Produces: proof the spec's verification section holds.

- [ ] **Step 1: Fresh clone builds with no bootstrap step**

```bash
S="$USERPROFILE/AppData/Local/Temp/ovpn3-fresh"; rm -rf "$S"
git clone /d/weaver-sync/development/personal/openvpn3-go "$S"
cd "$S"
export CGO_CPPFLAGS="-IC:/Users/dyamm/scoop/apps/mingw/current/opt/include"
export CGO_LDFLAGS="-LC:/Users/dyamm/scoop/apps/mingw/current/opt/lib"
CGO_ENABLED=1 go build ./...
CGO_ENABLED=0 go test -short ./...
```

Expected: cgo build succeeds (fresh clone has NO `cgo_openssl_local.go` — the env vars are the documented consumer path); tests `ok`. This is the spec's "fresh clone, no bootstrap" gate.

- [ ] **Step 2: Rebuild the proxy from current HEAD**

```bash
cd /d/weaver-sync/development/personal/openvpn3-go
rm -rf dist/goproxy && task module:verify
```

Expected: `module zip OK` + `proxy ready: GOPROXY=file:///D:/weaver-sync/development/personal/openvpn3-go/dist/goproxy`

- [ ] **Step 3: Scratch consumer builds from the read-only module cache**

```bash
C="$USERPROFILE/AppData/Local/Temp/ovpn3-consumer"; rm -rf "$C"; mkdir -p "$C"; cd "$C"
cat > main.go <<'EOF'
package main

import (
	"fmt"

	openvpn3 "github.com/inovacc/openvpn3-go"
)

func main() {
	fmt.Println("engine available:", openvpn3.Available())
}
EOF
go mod init example.com/consumer
export GOMODCACHE="$C/modcache"
export GOPROXY="file:///D:/weaver-sync/development/personal/openvpn3-go/dist/goproxy"
export GOSUMDB=off
export GOFLAGS=-mod=mod
go get github.com/inovacc/openvpn3-go@v0.1.1
export CGO_CPPFLAGS="-IC:/Users/dyamm/scoop/apps/mingw/current/opt/include"
export CGO_LDFLAGS="-LC:/Users/dyamm/scoop/apps/mingw/current/opt/lib"
CGO_ENABLED=1 go build -o consumer.exe .
./consumer.exe
```

Expected: `go get` resolves v0.1.1 from the file proxy; the cgo build compiles the vendored core FROM THE READ-ONLY MODULE CACHE (several minutes); `./consumer.exe` prints `engine available: true`. This is the spec's decisive reusability proof.

- [ ] **Step 4: CI workflows sanity**

Read the three workflow files. Decision rule: jobs on `ubuntu-latest`/`macos-latest` are safe as-is (no cgo-tagged files build there, `.cpp` files are ignored when no cgo Go file matches the platform). If any job runs build/test on `windows-latest` WITHOUT `CGO_ENABLED: 0`, add `env: CGO_ENABLED: "0"` to that job (GitHub's Windows runners have no OpenSSL dev install; the cgo path is covered by Steps 1–3 locally) and commit:

```bash
git add .github/workflows
git commit -m "ci: pin CGO_ENABLED=0 on windows runners (cgo path verified locally)"
```

If all Windows jobs already disable cgo (or none exist), no change — skip the commit.

- [ ] **Step 5: Full gate + cleanup**

```bash
cd /d/weaver-sync/development/personal/openvpn3-go
task check
rm -rf "$USERPROFILE/AppData/Local/Temp/ovpn3-fresh" "$USERPROFILE/AppData/Local/Temp/ovpn3-consumer"
```

Expected: fmt + vet + lint + tests all clean. If `task lint` flags the new `tools/` code, fix the specific findings (they are new files — style findings are legitimate) and amend into a `chore: lint fixes for tools module` commit.
