// Package cli defines the enza-seal commands.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
	root.AddCommand(newInit(), newEncrypt(), newDecrypt(), newWhoami(), newTeam(), newDevice(),
		newKey(), newMenu(version), newUpdate(version), newUninstall())
	// Plain `enza-seal` in a terminal opens the menu; otherwise show help.
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		if isTerminal(cmd.InOrStdin()) {
			return runMenu(cmd, version)
		}
		return cmd.Help()
	}
	return root
}

// maybeAutoSync refreshes the cached team file when it is over a day old, so
// new colleagues and devices show up without anyone running a command. A
// failure is reported but never blocks the command.
func maybeAutoSync() {
	if os.Getenv("ENZA_SEAL_TEAM") != "" || os.Getenv("ENZA_SEAL_NO_AUTOSYNC") != "" {
		return
	}
	url := teamURL("")
	path, err := config.TeamCachePath()
	if url == "" || err != nil {
		return
	}
	if fi, err := os.Stat(path); err == nil && time.Since(fi.ModTime()) < 24*time.Hour {
		return
	}
	if _, err := team.SyncWithTimeout(url, path, 5*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "note: could not refresh the team file (%v)\n", err)
	}
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
		maybeAutoSync()
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
	if cfg.Device == "" {
		return nil, nil, errors.New("this device is encrypt-only and holds no private key; decrypt on a device that has one")
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
