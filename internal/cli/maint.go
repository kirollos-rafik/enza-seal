package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kirollos-rafik/enza-seal/internal/config"
	"github.com/kirollos-rafik/enza-seal/internal/keystore"
)

const (
	releaseAPI = "https://api.github.com/repos/kirollos-rafik/enza-seal/releases/latest"
	rawBase    = "https://raw.githubusercontent.com/kirollos-rafik/enza-seal/main"
)

func installCommand() string {
	if runtime.GOOS == "windows" {
		return "irm " + rawBase + "/install.ps1 | iex"
	}
	return "curl -fsSL " + rawBase + "/install.sh | sh"
}

func newUpdate(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check for a newer release and show how to install it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			api := os.Getenv("ENZA_SEAL_RELEASE_API")
			if api == "" {
				api = releaseAPI
			}
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(api)
			if err != nil {
				return fmt.Errorf("could not check for updates: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				fmt.Fprintln(cmd.OutOrStdout(), "No release has been published yet.")
				return nil
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("could not check for updates: HTTP %d", resp.StatusCode)
			}
			var rel struct {
				Tag string `json:"tag_name"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
				return err
			}
			latest := strings.TrimPrefix(rel.Tag, "v")
			w := cmd.OutOrStdout()
			if latest == strings.TrimPrefix(version, "v") {
				fmt.Fprintf(w, "You are on the latest version (%s).\n", version)
				return nil
			}
			fmt.Fprintf(w, "You have %s; the latest is %s. To update, run:\n\n  %s\n", version, latest, installCommand())
			return nil
		},
	}
}

func newUninstall() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove this device's key and settings (then delete the program file)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, cfgErr := config.Load()
			if cfgErr != nil && !errors.Is(cfgErr, config.ErrNotSetUp) {
				return cfgErr
			}
			e := cmd.ErrOrStderr()
			if !yes {
				fmt.Fprintln(e, "This deletes this device's private key and settings. Data sealed for this device can then no longer be opened here.")
				ans, err := prompt(bufio.NewReader(cmd.InOrStdin()), e, "Type yes to continue: ")
				if err != nil || strings.ToLower(ans) != "yes" {
					return errors.New("cancelled")
				}
			}
			if cfgErr == nil && cfg.Device != "" {
				if err := keystore.Delete(cfg.Device); err != nil {
					return err
				}
			}
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			fmt.Fprintln(e, "Key and settings removed.")
			if exe, err := os.Executable(); err == nil {
				fmt.Fprintf(e, "Last step: delete the program itself: %s\n", exe)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask for confirmation")
	return cmd
}
