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
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "modverify:", err)
		os.Exit(1)
	}
}

func run() error {
	version := flag.String("version", "", "semver to stamp, e.g. v0.1.1 (required)")
	proxy := flag.String("proxy", "", "emit a file:// GOPROXY layout into this dir")
	flag.Parse()
	if *version == "" || !strings.HasPrefix(*version, "v") {
		return fmt.Errorf("-version vX.Y.Z is required")
	}

	root, err := gitToplevel()
	if err != nil {
		return err
	}

	// 1. Export HEAD to a temp dir via git archive.
	export, err := os.MkdirTemp("", "modverify-export")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(export) }()
	archive := exec.Command("git", "-C", root, "archive", "--format=tar", "HEAD")
	untar := exec.Command("tar", "-x", "-C", export)
	untar.Stdin, err = archive.StdoutPipe()
	if err != nil {
		return fmt.Errorf("git archive stdout pipe: %w", err)
	}
	untar.Stderr = os.Stderr
	if err := untar.Start(); err != nil {
		return err
	}
	if err := archive.Run(); err != nil {
		return fmt.Errorf("git archive: %w", err)
	}
	if err := untar.Wait(); err != nil {
		return fmt.Errorf("untar archive: %w", err)
	}

	// 2. Build + validate the module zip.
	mv := module.Version{Path: modPath, Version: *version}
	zipPath := filepath.Join(os.TempDir(), "openvpn3-go-"+*version+".zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	if err := zip.CreateFromDir(f, mv, export); err != nil {
		return fmt.Errorf("module zip validation failed: %w", err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	fi, err := os.Stat(zipPath)
	if err != nil {
		return err
	}
	fmt.Printf("module zip OK: %s (%.1f MB)\n", zipPath, float64(fi.Size())/1e6)

	// 3. Optional file:// proxy layout.
	if *proxy == "" {
		return nil
	}
	vdir := filepath.Join(*proxy, filepath.FromSlash(modPath), "@v")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		return err
	}
	gomod, err := os.ReadFile(filepath.Join(export, "go.mod"))
	if err != nil {
		return err
	}
	info, err := json.Marshal(map[string]string{"Version": *version, "Time": "2026-07-02T00:00:00Z"})
	if err != nil {
		return err
	}
	zipBytes, err := os.ReadFile(zipPath)
	if err != nil {
		return err
	}
	for name, content := range map[string][]byte{
		"list":             []byte(*version + "\n"),
		*version + ".info": info,
		*version + ".mod":  gomod,
		*version + ".zip":  zipBytes,
	} {
		if err := os.WriteFile(filepath.Join(vdir, name), content, 0o644); err != nil {
			return err
		}
	}
	abs, err := filepath.Abs(*proxy)
	if err != nil {
		return err
	}
	fmt.Printf("proxy ready: GOPROXY=file:///%s\n", filepath.ToSlash(abs))
	return nil
}

func gitToplevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolve git toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
