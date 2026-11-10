// Package cli defines the enza-seal commands.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kirollos-rafik/enza-seal/internal/config"
	"github.com/kirollos-rafik/enza-seal/internal/keystore"
	"github.com/kirollos-rafik/enza-seal/internal/team"
)

// DefaultTeamURL is where `team sync` downloads the team file from. It is set
// at build time with -ldflags; ENZA_SEAL_TEAM_URL and --url override it.
var DefaultTeamURL = "https://raw.githubusercontent.com/kirollos-rafik/enza-seal/main/team/recipients.json"

func NewRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "enza-seal",
		Short:         "Share test data safely: encrypt it for named colleagues",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newInit(), newEncrypt(), newDecrypt(), newWhoami(), newTeam(), newDevice())
	return root
}

// ---- shared helpers ----

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// loadTeam reads the team file: ENZA_SEAL_TEAM (a path) or the synced cache.
func loadTeam() (*team.File, error) {
	p := os.Getenv("ENZA_SEAL_TEAM")
	if p == "" {
		var err error
		if p, err = config.TeamCachePath(); err != nil {
			return nil, err
		}
	}
	f, err := team.Load(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("no team file yet: run enza-seal team sync")
	}
	return f, err
}

// loadIdentity returns this device's private key from the OS secure store.
func loadIdentity() (*config.Config, age.Identity, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	secret, err := keystore.Load(cfg.Device)
	if err != nil {
		return nil, nil, err
	}
	id, err := age.ParseX25519Identity(secret)
	if err != nil {
		return nil, nil, fmt.Errorf("the stored key is damaged: %w", err)
	}
	return cfg, id, nil
}

// prompt asks a question on stderr and reads one line from in.
func prompt(in *bufio.Reader, out io.Writer, label string) (string, error) {
	fmt.Fprint(out, label)
	line, err := in.ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func teamURL(flag string) string {
	switch {
	case flag != "":
		return flag
	case os.Getenv("ENZA_SEAL_TEAM_URL") != "":
		return os.Getenv("ENZA_SEAL_TEAM_URL")
	}
	return DefaultTeamURL
}
