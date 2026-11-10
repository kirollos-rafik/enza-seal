// Package team reads the team address book (team/recipients.json): people,
// their device public keys, and groups. It holds public keys only.
package team

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
)

type Person struct {
	Name    string            `json:"name"`
	Team    string            `json:"team,omitempty"`
	GitHub  string            `json:"github,omitempty"`
	Devices map[string]string `json:"devices"`
}

type File struct {
	Version int                 `json:"version"`
	People  map[string]Person   `json:"people"`
	Groups  map[string][]string `json:"groups,omitempty"`
}

// Parse reads and validates a team file: version, every key, every group member.
func Parse(b []byte) (*File, error) {
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("team file is not valid JSON: %w", err)
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("unsupported team file version %d (this tool reads version 1)", f.Version)
	}
	for id, p := range f.People {
		for dev, key := range p.Devices {
			if _, err := age.ParseX25519Recipient(key); err != nil {
				return nil, fmt.Errorf("person %q device %q has an invalid public key", id, dev)
			}
		}
	}
	for g, members := range f.Groups {
		for _, m := range members {
			if _, ok := f.People[m]; !ok {
				return nil, fmt.Errorf("group %q lists unknown person %q", g, m)
			}
		}
	}
	return &f, nil
}

// Resolve turns a --to value (a person, "@group", or "me") into public keys,
// one per device, without duplicates. me is the caller's own person id.
func (f *File) Resolve(spec, me string) ([]string, error) {
	spec = strings.TrimSpace(spec)
	var people []string
	switch {
	case spec == "me":
		if me == "" {
			return nil, errors.New(`"me" needs setup first: run enza-seal init`)
		}
		people = []string{me}
	case strings.HasPrefix(spec, "@"):
		members, ok := f.Groups[strings.TrimPrefix(spec, "@")]
		if !ok {
			return nil, fmt.Errorf("unknown group %q (see: enza-seal team list)", spec)
		}
		people = members
	default:
		people = []string{spec}
	}
	seen := map[string]bool{}
	var keys []string
	for _, id := range people {
		p, ok := f.People[id]
		if !ok {
			return nil, fmt.Errorf("unknown person %q (see: enza-seal team list)", id)
		}
		if len(p.Devices) == 0 {
			return nil, fmt.Errorf("%q has no registered devices yet", id)
		}
		devs := make([]string, 0, len(p.Devices))
		for d := range p.Devices {
			devs = append(devs, d)
		}
		sort.Strings(devs)
		for _, d := range devs {
			if k := p.Devices[d]; !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	return keys, nil
}

// PersonIDs and GroupIDs return sorted ids for listing.
func (f *File) PersonIDs() []string { return sortedKeys(f.People) }
func (f *File) GroupIDs() []string  { return sortedKeys(f.Groups) }

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Load reads and validates a team file from disk.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Sync downloads the team file from url, validates it, and replaces the
// cached copy at path only if it is valid.
func Sync(url, path string) (*File, error) {
	if url == "" {
		return nil, errors.New("no team file location set: pass --url or set ENZA_SEAL_TEAM_URL")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("could not download the team file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not download the team file: HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	f, err := Parse(b)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return nil, err
	}
	return f, os.Rename(tmp, path)
}
