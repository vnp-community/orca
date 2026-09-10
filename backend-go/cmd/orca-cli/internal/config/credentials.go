// Package config manages orca-cli's on-disk credentials — the JWT
// /auth/cli-token issues, cached so every command doesn't re-login.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Credentials is the on-disk shape at ~/.config/orca/credentials.json —
// 0600, since it holds a live bearer JWT.
type Credentials struct {
	APIURL    string    `json:"api_url"`
	JWT       string    `json:"jwt"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Path returns ~/.config/orca/credentials.json, honoring $XDG_CONFIG_HOME
// when set (cross-platform per AGENTS.md — os.UserConfigDir already
// resolves the right base dir on macOS/Linux/Windows).
func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "orca", "credentials.json"), nil
}

// Load reads Credentials from disk. A missing file returns the zero value
// and a nil error — "not logged in yet" is not a hard failure.
func Load() (Credentials, error) {
	path, err := Path()
	if err != nil {
		return Credentials{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Credentials{}, nil
	}
	if err != nil {
		return Credentials{}, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, err
	}
	return creds, nil
}

// Save writes creds to disk with 0600 permissions — never world/group
// readable, since this file holds a live bearer JWT.
func Save(creds Credentials) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ResolveAPIURL applies the ORCA_API_URL env override, falling back to
// creds.APIURL, falling back to def.
func ResolveAPIURL(creds Credentials, def string) string {
	if v := os.Getenv("ORCA_API_URL"); v != "" {
		return v
	}
	if creds.APIURL != "" {
		return creds.APIURL
	}
	return def
}

// ResolveToken applies the ORCA_API_TOKEN env override, falling back to
// creds.JWT.
func ResolveToken(creds Credentials) string {
	if v := os.Getenv("ORCA_API_TOKEN"); v != "" {
		return v
	}
	return creds.JWT
}
