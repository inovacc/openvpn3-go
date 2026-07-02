package openvpn3

import (
	"os"
	"strings"
	"testing"
)

func TestConfigFromOVPN_KeepsInlineContent(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample.ovpn")
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := ConfigFromOVPN(string(raw), Credentials{User: "u", Pass: "p"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if !strings.Contains(cfg.Content, "remote vpn.example.com 1194") {
		t.Fatal("content must preserve remote directive")
	}

	if cfg.Username != "u" || cfg.Password != "p" {
		t.Fatalf("creds not mapped: %+v", cfg)
	}
}

func TestConfigFromOVPN_RejectsEmpty(t *testing.T) {
	if _, err := ConfigFromOVPN("   \n", Credentials{}); err == nil {
		t.Fatal("empty profile must error")
	}
}
