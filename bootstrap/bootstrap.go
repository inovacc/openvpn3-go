// Package bootstrap initializes and downloads the C/C++ build dependencies the
// cgo OpenVPN3 engine needs, per operating system. It is the Go replacement for
// the former lensr scripts/openvpn3/build-lensrd-windows.ps1 dependency step.
//
// Because the cgo #cgo directives use ${SRCDIR}-relative include paths, the
// dependencies (the OpenVPN3 submodule + asio/lz4/jsoncpp[/tap-windows]) and the
// generated machine-specific OpenSSL flags file must live in the MODULE root
// directory. That requires a writable on-disk module — i.e. consumers wire this
// module via a local `replace`, which is also why it lives in a sync folder.
//
// Usage: go run github.com/inovacc/openvpn3-go/cmd/openvpn
package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Options tunes the bootstrap. The zero value is the default (auto-detected
// module root, default toolchain discovery).
type Options struct {
	// ModuleRoot overrides where deps are written. Empty = auto-detect from the
	// bootstrap source file location (works when built from source via replace /
	// go.work, which is the supported consumption model).
	ModuleRoot string
	// Force re-fetches dependencies even when their markers are present.
	Force bool
	// Logf receives progress lines. nil => os.Stderr.
	Logf func(format string, args ...any)
}

func (o Options) logf(format string, args ...any) {
	if o.Logf != nil {
		o.Logf(format, args...)
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// dep is one pinned git dependency fetched into <root>/deps/<Name>.
type dep struct {
	Name   string
	Tag    string
	URL    string
	Marker string // relative to the dep dir; presence => already fetched
}

// commonDeps are the header/source deps every OS needs.
var commonDeps = []dep{
	{Name: "asio", Tag: "asio-1-24-0", URL: "https://github.com/chriskohlhoff/asio.git", Marker: filepath.Join("asio", "include", "asio.hpp")},
	{Name: "lz4", Tag: "v1.8.3", URL: "https://github.com/lz4/lz4.git", Marker: filepath.Join("lib", "lz4.h")},
	{Name: "jsoncpp", Tag: "1.8.4", URL: "https://github.com/open-source-parsers/jsoncpp.git", Marker: filepath.Join("include", "json", "json.h")},
}

const openvpn3SubmoduleURL = "https://github.com/OpenVPN/openvpn3"

// openvpn3PinnedSHA is the exact OpenVPN3 core commit lensr built + shipped
// against (recovered from the former git submodule gitlink). It is pinned
// because newer master commits add a libfmt (<fmt/format.h>) dependency the
// vendored dep set does not provide; this commit builds with just
// asio/lz4/jsoncpp + OpenSSL.
const openvpn3PinnedSHA = "5b7841a847619e9e1ba3f7371e0c9e2743383481"

// Run performs the per-OS bootstrap: resolves the module root, fetches the
// OpenVPN3 submodule + common C/C++ deps, then runs the OS-specific step
// (toolchain + OpenSSL detection, generated cgo flags, platform deps).
func Run(opts Options) error {
	root := opts.ModuleRoot
	if root == "" {
		r, err := moduleRoot()
		if err != nil {
			return err
		}
		root = r
	}
	opts.logf("openvpn3-bootstrap: module root = %s (GOOS=%s)", root, runtime.GOOS)

	if err := ensureOpenVPN3(root, opts); err != nil {
		return err
	}
	for _, d := range commonDeps {
		if err := fetchDep(root, d, opts); err != nil {
			return err
		}
	}

	return bootstrapOS(root, opts)
}

// moduleRoot resolves the module directory from this source file's location.
// bootstrap/bootstrap.go -> parent of bootstrap/ is the module root. This works
// when the module is built from source (replace / go.work), the supported model.
func moduleRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("openvpn3-bootstrap: cannot resolve module root (runtime.Caller failed)")
	}
	// file = <root>/bootstrap/bootstrap.go
	return filepath.Dir(filepath.Dir(file)), nil
}

// ensureOpenVPN3 fetches the OpenVPN3 C++ core PINNED to openvpn3PinnedSHA into
// <root>/openvpn. If a checkout already exists at the wrong commit (e.g. a prior
// master clone), it is re-fetched. Uses fetch-by-SHA (GitHub allows reachable
// SHA fetches) so the depth-1 checkout lands exactly on the pin.
func ensureOpenVPN3(root string, opts Options) error {
	dst := filepath.Join(root, "openvpn")
	marker := filepath.Join(dst, "client", "ovpncli.hpp")

	if !opts.Force && fileExists(marker) && headSHA(dst) == openvpn3PinnedSHA {
		opts.logf("[skip] openvpn3 core present @ %s", openvpn3PinnedSHA)
		return nil
	}

	opts.logf("[fetch] openvpn3 core @ %s -> %s", openvpn3PinnedSHA, dst)
	_ = os.RemoveAll(dst)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := run(dst, "git", "init", "-q"); err != nil {
		return err
	}
	if err := run(dst, "git", "remote", "add", "origin", openvpn3SubmoduleURL); err != nil {
		return err
	}
	if err := run(dst, "git", "fetch", "--depth", "1", "origin", openvpn3PinnedSHA); err != nil {
		return err
	}
	return run(dst, "git", "checkout", "-q", "FETCH_HEAD")
}

// headSHA returns the current HEAD commit of a git checkout, or "" on error.
func headSHA(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// fetchDep git-clones a pinned dep into <root>/deps/<Name> (idempotent).
func fetchDep(root string, d dep, opts Options) error {
	depsDir := filepath.Join(root, "deps")
	if err := os.MkdirAll(depsDir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(depsDir, d.Name)
	if !opts.Force && fileExists(filepath.Join(target, d.Marker)) {
		opts.logf("[skip] %s present", d.Name)
		return nil
	}
	_ = os.RemoveAll(target)
	opts.logf("[fetch] %s @ %s", d.Name, d.Tag)
	return run(root, "git", "clone", "--depth", "1", "--branch", d.Tag, d.URL, target)
}

// run executes a command in dir, streaming output to stderr.
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

func fileExists(p string) bool { fi, err := os.Stat(p); return err == nil && !fi.IsDir() }
