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
