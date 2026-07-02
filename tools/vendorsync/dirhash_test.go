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
