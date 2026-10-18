// Package config keeps the app's settings in %APPDATA%\Warchest Companion\config.json.
// The device key is encrypted with Windows' own per-user protection (DPAPI), so another
// Windows account on the same PC (or a copied file) can't use it.
package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	WoWDir      string `json:"wow_dir,omitempty"` // the folder with Logs\WoWChatLog.txt in it (or the game folder above it)
	TokenSealed string `json:"token,omitempty"`   // the device key, encrypted for this Windows user
	Account     string `json:"account,omitempty"` // shown in the menu
	Paused      bool   `json:"paused,omitempty"`  // relaying switched off by the player
	SkipVersion string `json:"skip_version,omitempty"`
}

func Dir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "Warchest Companion")
}

func path() string { return filepath.Join(Dir(), "config.json") }

func Load() *Config {
	c := &Config{}
	if data, err := os.ReadFile(path()); err == nil {
		_ = json.Unmarshal(data, c)
	}
	return c
}

func (c *Config) Save() error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	tmp := path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path())
}

// Token returns the device key ("" when not connected).
func (c *Config) Token() string {
	if c.TokenSealed == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(c.TokenSealed)
	if err != nil {
		return ""
	}
	plain, err := unseal(raw)
	if err != nil {
		return ""
	}
	return string(plain)
}

func (c *Config) SetToken(tok string) error {
	if tok == "" {
		c.TokenSealed = ""
		return nil
	}
	sealed, err := seal([]byte(tok))
	if err != nil {
		return err
	}
	c.TokenSealed = base64.StdEncoding.EncodeToString(sealed)
	return nil
}
