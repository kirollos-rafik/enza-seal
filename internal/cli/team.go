package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kirollos-rafik/enza-seal/internal/config"
	"github.com/kirollos-rafik/enza-seal/internal/crypto"
	"github.com/kirollos-rafik/enza-seal/internal/team"
)

func newTeam() *cobra.Command {
	cmd := &cobra.Command{Use: "team", Short: "See who you can encrypt to, and refresh the team file"}

	var url string
	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Download the latest team file (public keys) from the repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.TeamCachePath()
			if err != nil {
				return err
			}
			f, err := team.Sync(teamURL(url), path)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Team file updated: %d people, %d groups.\n", len(f.People), len(f.Groups))
			return nil
		},
	}
	syncCmd.Flags().StringVar(&url, "url", "", "team file URL (default: build setting or ENZA_SEAL_TEAM_URL)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List people and groups you can encrypt to",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := loadTeam()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "People:")
			for _, id := range f.PersonIDs() {
				p := f.People[id]
				fmt.Fprintf(w, "  %-14s %s", id, p.Name)
				if p.Team != "" {
					fmt.Fprintf(w, " (%s)", p.Team)
				}
				fmt.Fprintf(w, "  [%d device(s)]\n", len(p.Devices))
			}
			if len(f.Groups) > 0 {
				fmt.Fprintln(w, "Groups:")
				for _, g := range f.GroupIDs() {
					fmt.Fprintf(w, "  @%-13s %s\n", g, strings.Join(f.Groups[g], ", "))
				}
			}
			return nil
		},
	}
	cmd.AddCommand(syncCmd, listCmd)
	return cmd
}

func newDevice() *cobra.Command {
	cmd := &cobra.Command{Use: "device", Short: "Your devices (laptop, AWS WorkSpace, ...)"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Show your registered devices and their fingerprints",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			f, err := loadTeam()
			if err != nil {
				fmt.Fprintf(w, "This device: %s  %s\n(team file not available: %v)\n", cfg.Device, crypto.Fingerprint(cfg.PublicKey), err)
				return nil
			}
			p, ok := f.People[cfg.Person]
			if !ok {
				fmt.Fprintf(w, "You are not in the team file yet. This device: %s  %s\n", cfg.Device, crypto.Fingerprint(cfg.PublicKey))
				return nil
			}
			names := make([]string, 0, len(p.Devices))
			for d := range p.Devices {
				names = append(names, d)
			}
			sort.Strings(names)
			for _, d := range names {
				mark := ""
				if d == cfg.Device && p.Devices[d] == cfg.PublicKey {
					mark = "  <- this device"
				}
				fmt.Fprintf(w, "  %-12s %s%s\n", d, crypto.Fingerprint(p.Devices[d]), mark)
			}
			return nil
		},
	})
	return cmd
}
