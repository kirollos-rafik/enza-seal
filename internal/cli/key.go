package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kirollos-rafik/enza-seal/internal/config"
	"github.com/kirollos-rafik/enza-seal/internal/crypto"
	"github.com/kirollos-rafik/enza-seal/internal/keystore"
)

// scryptWorkFactor is age's default (about one second); tests lower it.
var scryptWorkFactor = 18

func newKey() *cobra.Command {
	cmd := &cobra.Command{Use: "key", Short: "Back up, restore or replace this device's key"}
	cmd.AddCommand(newKeyBackup(), newKeyRestore(), newKeyRotate())
	return cmd
}

// readPassphrase asks twice when confirm is set. On a terminal the input is
// hidden; from a pipe (used by tests and scripts) it reads one line per ask.
func readPassphrase(cmd *cobra.Command, in *bufio.Reader, label string, confirm bool) (string, error) {
	ask := func(l string) (string, error) {
		if f, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			fmt.Fprint(cmd.ErrOrStderr(), l)
			b, err := term.ReadPassword(int(f.Fd()))
			fmt.Fprintln(cmd.ErrOrStderr())
			return string(b), err
		}
		return prompt(in, io.Discard, l)
	}
	p, err := ask(label)
	if err != nil {
		return "", err
	}
	if len(p) < 8 {
		return "", errors.New("passphrase must be at least 8 characters")
	}
	if confirm {
		p2, err := ask("Repeat the passphrase: ")
		if err != nil {
			return "", err
		}
		if p != p2 {
			return "", errors.New("passphrases do not match")
		}
	}
	return p, nil
}

func newKeyBackup() *cobra.Command {
	var out string
	var force bool
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Save this device's private key to a file protected by a passphrase you choose",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if cfg.Device == "" {
				return errors.New("this device has no private key to back up")
			}
			if out == "" {
				return errors.New("choose where to save it with --out <file>")
			}
			secret, err := keystore.Load(cfg.Device)
			if err != nil {
				return err
			}
			pass, err := readPassphrase(cmd, bufio.NewReader(cmd.InOrStdin()), "Backup passphrase (min 8 characters): ", true)
			if err != nil {
				return err
			}
			rcpt, err := age.NewScryptRecipient(pass)
			if err != nil {
				return err
			}
			rcpt.SetWorkFactor(scryptWorkFactor)
			var buf bytes.Buffer
			if err := crypto.Encrypt(&buf, strings.NewReader(secret), []age.Recipient{rcpt}, true); err != nil {
				return err
			}
			if err := writeNew(out, buf.Bytes(), force); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Backup written to %s. Keep it somewhere safe, apart from the passphrase.\n", out)
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "file to write the backup to")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the file if it exists")
	return cmd
}

func newKeyRestore() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a key from a backup file into this device's secure store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("%w (run init first so this device has a name, then restore)", err)
			}
			if cfg.Device == "" {
				return errors.New("this device is encrypt-only; run init --force without --encrypt-only first")
			}
			if file == "" {
				return errors.New("say which backup with --file <file>")
			}
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()
			pass, err := readPassphrase(cmd, bufio.NewReader(cmd.InOrStdin()), "Backup passphrase: ", false)
			if err != nil {
				return err
			}
			idn, err := age.NewScryptIdentity(pass)
			if err != nil {
				return err
			}
			b, err := crypto.DecryptAll(f, idn)
			if err != nil {
				return errors.New("could not open the backup: wrong passphrase or damaged file")
			}
			secret := strings.TrimSpace(string(b))
			id, err := age.ParseX25519Identity(secret)
			if err != nil {
				return errors.New("the backup does not contain a valid key")
			}
			if err := keystore.Save(cfg.Device, secret); err != nil {
				return err
			}
			cfg.PublicKey = id.Recipient().String()
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Key restored. Fingerprint: %s\n", crypto.Fingerprint(cfg.PublicKey))
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "backup file to restore")
	return cmd
}

func newKeyRotate() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate",
		Short: "Create a new key for this device; you then replace the old public key in the team file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if cfg.Device == "" {
				return errors.New("this device has no private key to rotate")
			}
			id, err := crypto.GenerateIdentity()
			if err != nil {
				return err
			}
			if err := keystore.Save(cfg.Device, id.String()); err != nil {
				return err
			}
			cfg.PublicKey = id.Recipient().String()
			if err := config.Save(cfg); err != nil {
				return err
			}
			e := cmd.ErrOrStderr()
			fmt.Fprintln(e, "New key created. Messages sealed for the OLD key can no longer be opened on this device.")
			fmt.Fprintf(e, "Replace this device's key in team/recipients.json (pull request):\n  %q: %q\nFingerprint: %s\n",
				cfg.Device, cfg.PublicKey, crypto.Fingerprint(cfg.PublicKey))
			return nil
		},
	}
}
