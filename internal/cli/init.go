package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kirollos-rafik/enza-seal/internal/config"
	"github.com/kirollos-rafik/enza-seal/internal/crypto"
	"github.com/kirollos-rafik/enza-seal/internal/keystore"
	"github.com/kirollos-rafik/enza-seal/internal/team"
)

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)

func newInit() *cobra.Command {
	var name, person, teamName, device string
	var force, encryptOnly bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "One-time setup: create this device's key and store it in the OS secure store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			errOut := cmd.ErrOrStderr()
			in := bufio.NewReader(cmd.InOrStdin())

			if existing, err := config.Load(); err == nil && !force {
				return fmt.Errorf("already set up as %s (device %q); use --force to replace the key", existing.Person, existing.Device)
			}

			// Best effort: if a team file is available, reuse the details already
			// registered for this person (adding a second device).
			var tf *team.File
			if f, err := loadTeam(); err == nil {
				tf = f
			}

			var err error
			if person == "" {
				if person, err = prompt(in, errOut, "Your short id (e.g. alice): "); err != nil {
					return err
				}
			}
			if !idPattern.MatchString(person) {
				return errors.New("id must be lowercase letters, digits, dot, dash or underscore (max 32)")
			}
			existing, known := team.Person{}, false
			if tf != nil {
				existing, known = tf.People[person]
			}
			if known {
				fmt.Fprintf(errOut, "Found you in the team file: %s (%d device(s)).\n", existing.Name, len(existing.Devices))
				if name == "" {
					name = existing.Name
				}
				if teamName == "" {
					teamName = existing.Team
				}
			}
			if name == "" {
				if name, err = prompt(in, errOut, "Your name: "); err != nil {
					return err
				}
			}
			if teamName == "" && !known {
				if teamName, err = prompt(in, errOut, "Your team: "); err != nil {
					return err
				}
			}
			if device == "" {
				device = "laptop"
			}
			if !idPattern.MatchString(device) {
				return errors.New("device name must be lowercase letters, digits, dot, dash or underscore (max 32)")
			}
			if known && !encryptOnly {
				if _, dup := existing.Devices[device]; dup {
					return fmt.Errorf("device %q is already registered for %s; choose another name with --device", device, person)
				}
			}

			if encryptOnly {
				if !known {
					return errors.New("--encrypt-only needs you to be in the team file already (run enza-seal team sync)")
				}
				if err := config.Save(&config.Config{Person: person, Name: name, Team: teamName}); err != nil {
					return err
				}
				fmt.Fprintln(errOut, "Set up as encrypt-only: no private key is stored on this device.")
				fmt.Fprintln(errOut, "Encrypting works (including --to me); decrypt on a device that has a key.")
				return nil
			}

			id, err := crypto.GenerateIdentity()
			if err != nil {
				return err
			}
			if err := keystore.Save(device, id.String()); err != nil {
				return err
			}
			pub := id.Recipient().String()
			if err := config.Save(&config.Config{Person: person, Name: name, Team: teamName, Device: device, PublicKey: pub}); err != nil {
				return err
			}

			fmt.Fprintf(errOut, "\nPrivate key saved in the OS secure store (never leaves this device).\n")
			fmt.Fprintf(errOut, "Public key:   %s\nFingerprint:  %s\n\n", pub, crypto.Fingerprint(pub))
			fmt.Fprintln(errOut, "Last step: add this to team/recipients.json in the enza-seal repo (open a pull request).")
			fmt.Fprintln(errOut, "Confirm the fingerprint with your team lead by voice or in person.")
			fmt.Fprintln(errOut)
			fmt.Fprintln(cmd.OutOrStdout(), registrationSnippet(person, name, teamName, device, pub, known))
			return nil
		},
	}
	cmd.Flags().StringVar(&person, "person", "", "your short id, e.g. alice")
	cmd.Flags().StringVar(&name, "name", "", "your display name")
	cmd.Flags().StringVar(&teamName, "team", "", "your team")
	cmd.Flags().StringVar(&device, "device", "", `name of this device (default "laptop"; use e.g. "workspace" for AWS WorkSpaces)`)
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing key on this device")
	cmd.Flags().BoolVar(&encryptOnly, "encrypt-only", false, "no private key on this device (e.g. non-persistent WorkSpaces); it can encrypt but not decrypt")
	return cmd
}

func registrationSnippet(person, name, teamName, device, pub string, known bool) string {
	if known {
		return fmt.Sprintf("Add under \"people\" > \"%s\" > \"devices\":\n  %q: %q", person, device, pub)
	}
	entry := map[string]any{"name": name, "devices": map[string]string{device: pub}}
	if teamName != "" {
		entry["team"] = teamName
	}
	b, _ := json.MarshalIndent(map[string]any{person: entry}, "  ", "  ")
	return "Add under \"people\":\n  " + strings.TrimSpace(string(b))
}

func newWhoami() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show who you are on this device and your public key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Name:         %s (%s)\n", cfg.Name, cfg.Person)
			if cfg.Team != "" {
				fmt.Fprintf(w, "Team:         %s\n", cfg.Team)
			}
			fmt.Fprintf(w, "Device:       %s\nPublic key:   %s\nFingerprint:  %s\n", cfg.Device, cfg.PublicKey, crypto.Fingerprint(cfg.PublicKey))
			return nil
		},
	}
}
