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
