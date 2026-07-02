package openvpn3

import (
	"fmt"
	"strings"
)

// Credentials are optional username/password for the profile.
type Credentials struct {
	User string
	Pass string
}

// ConfigFromOVPN builds a Config from inline .ovpn content. OpenVPN3 requires
// inline profiles; external file refs must already be merged by the caller
// (or by OpenVPN3's ProfileMerge before eval). This validates non-emptiness.
func ConfigFromOVPN(content string, creds Credentials) (Config, error) {
	if strings.TrimSpace(content) == "" {
		return Config{}, fmt.Errorf("%w: empty profile content", ErrEvalConfig)
	}

	return Config{
		Content:  content,
		Username: creds.User,
		Password: creds.Pass,
	}, nil
}
