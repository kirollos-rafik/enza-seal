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
)

const armorEnd = "-----END AGE ENCRYPTED FILE-----"

func newEncrypt() *cobra.Command {
	var to []string
	var file, out string
	var armor bool
	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt text or a file for named people, groups, or yourself (--to me)",
		Long: `Encrypt text or a file so only the named recipients can open it.

Text comes from a hidden prompt, or from a pipe. Plain text is never taken as a
command-line argument, so it cannot end up in your shell history.

  enza-seal encrypt --to bob
  enza-seal encrypt --to bob,carol
  enza-seal encrypt --to @payments-dev
  enza-seal encrypt --to bob -f cards.csv`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(to) == 0 {
				return errors.New("say who it is for with --to (a person, @group, or me)")
			}
			tf, err := loadTeam()
			if err != nil {
				return err
			}
			me := ""
			if cfg, err := config.Load(); err == nil {
				me = cfg.Person
			}
			var keys []string
			for _, spec := range to {
				k, err := tf.Resolve(spec, me)
				if err != nil {
					return err
				}
				keys = append(keys, k...)
			}
			rcpts, err := crypto.ParseRecipients(dedupe(keys))
			if err != nil {
				return err
			}

			var src io.Reader
			switch {
			case file != "":
				f, err := os.Open(file)
				if err != nil {
					return err
				}
				defer f.Close()
				src = f
			case isTerminal(cmd.InOrStdin()):
				f := cmd.InOrStdin().(*os.File)
				fmt.Fprint(cmd.ErrOrStderr(), "Text to encrypt (hidden, press Enter to finish): ")
				b, err := term.ReadPassword(int(f.Fd()))
				fmt.Fprintln(cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				src = bytes.NewReader(b)
			default:
				src = cmd.InOrStdin()
			}

			// Files are written binary (smaller); text is armored so it pastes
			// cleanly into chat, email and tickets.
			armored := file == "" || armor
			if out == "" && file != "" {
				out = file + ".seal"
			}
			dst := cmd.OutOrStdout()
			var tmpName string
			var tmp *os.File
			if out != "" {
				if tmp, err = os.CreateTemp(dirOf(out), ".enza-seal-*"); err != nil {
					return err
				}
				tmpName = tmp.Name()
				defer os.Remove(tmpName)
				dst = tmp
			}
			if err := crypto.Encrypt(dst, src, rcpts, armored); err != nil {
				return err
			}
			if tmp != nil {
				if err := tmp.Close(); err != nil {
					return err
				}
				if err := os.Rename(tmpName, out); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Written: %s\n", out)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Encrypted for %d device key(s).\n", len(rcpts))
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&to, "to", nil, "recipients: person, @group or me (comma separated)")
	cmd.Flags().StringVarP(&file, "file", "f", "", "encrypt this file (writes <file>.seal)")
	cmd.Flags().StringVar(&out, "out", "", "write the result to this file instead of the terminal")
	cmd.Flags().BoolVar(&armor, "armor", false, "paste-friendly text output even for files")
	return cmd
}

func newDecrypt() *cobra.Command {
	var file, out string
	var force bool
	cmd := &cobra.Command{
		Use:   "decrypt",
		Short: "Decrypt a sealed block or a .seal file with this device's key",
		Long: `Decrypt data that was encrypted for you.

  enza-seal decrypt                 (paste the block, it ends at the END line)
  enza-seal decrypt -f cards.csv.seal`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, id, err := loadIdentity()
			if err != nil {
				return err
			}

			if file != "" {
				return decryptFile(cmd, file, out, force, id)
			}

			var src io.Reader = cmd.InOrStdin()
			if isTerminal(src) {
				fmt.Fprintln(cmd.ErrOrStderr(), "Paste the sealed block (it ends at the END line):")
				src = readBlock(bufio.NewReader(src))
			}
			plain, err := crypto.DecryptAll(src, id)
			if err != nil {
				return err
			}
			if out != "" {
				return writeNew(out, plain, force)
			}
			_, err = cmd.OutOrStdout().Write(plain)
			return err
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "decrypt this .seal file")
	cmd.Flags().StringVar(&out, "out", "", "write the result to this file instead of the terminal")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the output file if it exists")
	return cmd
}

// decryptFile decrypts to a temp file and renames it into place only after the
// whole file authenticated, so a tampered file never leaves partial output.
func decryptFile(cmd *cobra.Command, file, out string, force bool, id age.Identity) error {
	if out == "" {
		if !strings.HasSuffix(file, ".seal") {
			return errors.New("cannot guess the output name: use --out <file>")
		}
		out = strings.TrimSuffix(file, ".seal")
	}
	if _, err := os.Stat(out); err == nil && !force {
		return fmt.Errorf("%s already exists; use --force to overwrite", out)
	}
	in, err := os.Open(file)
	if err != nil {
		return err
	}
	defer in.Close()
	pr, err := crypto.Decrypt(in, id)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dirOf(out), ".enza-seal-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, pr); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), out); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Written: %s\n", out)
	return nil
}

// readBlock reads pasted lines until the END line (or end of input).
func readBlock(r *bufio.Reader) io.Reader {
	var sb strings.Builder
	for {
		line, err := r.ReadString('\n')
		sb.WriteString(line)
		if strings.Contains(line, armorEnd) || err != nil {
			break
		}
	}
	return strings.NewReader(sb.String())
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func dirOf(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[:i+1]
	}
	return "."
}

func writeNew(path string, data []byte, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%s already exists; use --force to overwrite", path)
	}
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
