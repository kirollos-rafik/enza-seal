// Package config stores this device's identity (not the private key) in the
// user's config folder.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	Person    string `json:"person"`
	Name      string `json:"name"`
	Team      string `json:"team,omitempty"`
	Device    string `json:"device"`
	PublicKey string `json:"public_key"`
}

// Dir is the config folder. ENZA_SEAL_HOME overrides it (used by tests).
func Dir() (string, error) {
	if d := os.Getenv("ENZA_SEAL_HOME"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "enza-seal"), nil
}

func path(name string) (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name), nil
}

// TeamCachePath is where the synced team file is kept.
func TeamCachePath() (string, error) { return path("recipients.json") }

// ErrNotSetUp means init has not been run on this device.
var ErrNotSetUp = errors.New("not set up yet: run enza-seal init")

func Load() (*Config, error) {
	p, err := path("config.json")
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotSetUp
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func Save(c *Config) error {
	p, err := path("config.json")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}
