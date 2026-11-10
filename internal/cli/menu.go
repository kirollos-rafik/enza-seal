package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newMenu is the numbered menu for people who prefer not to remember flags.
// It simply runs the normal commands.
func newMenu(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "menu",
		Short: "Open the interactive menu (also what plain `enza-seal` does in a terminal)",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return runMenu(cmd, version) },
	}
}

func runMenu(cmd *cobra.Command, version string) error {
	in := bufio.NewReader(cmd.InOrStdin())
	out := cmd.ErrOrStderr()

	exec := func(args ...string) {
		c := NewRoot(version)
		c.SetIn(cmd.InOrStdin())
		c.SetOut(cmd.OutOrStdout())
		c.SetErr(out)
		c.SetArgs(args)
		if err := c.Execute(); err != nil {
			fmt.Fprintln(out, "error:", err)
		}
	}
	ask := func(label string) (string, bool) {
		s, err := prompt(in, out, label)
		return s, err == nil
	}
	recipients := func() (string, bool) {
		for {
			s, ok := ask("Who is it for? person, @group or me (comma separated, ? to list): ")
			if !ok {
				return "", false
			}
			if s == "?" {
				exec("team", "list")
				continue
			}
			return s, s != ""
		}
	}

	for {
		fmt.Fprint(out, `
enza-seal - share test data safely

  1) Encrypt text
  2) Encrypt a file
  3) Decrypt (clipboard or paste)
  4) Decrypt a file
  5) Team: list people and groups
  6) Team: refresh from the repo
  7) My key: show details
  q) Quit

`)
		choice, ok := ask("Choose: ")
		if !ok {
			return nil
		}
		switch strings.ToLower(choice) {
		case "1":
			if to, ok := recipients(); ok {
				exec("encrypt", "--to", to)
			}
		case "2":
			to, ok := recipients()
			if !ok {
				break
			}
			if path, ok := ask("File to encrypt: "); ok && path != "" {
				exec("encrypt", "--to", to, "-f", strings.Trim(path, `"`))
			}
		case "3":
			exec("decrypt")
		case "4":
			if path, ok := ask("File to decrypt (.seal): "); ok && path != "" {
				exec("decrypt", "-f", strings.Trim(path, `"`))
			}
		case "5":
			exec("team", "list")
		case "6":
			exec("team", "sync")
		case "7":
			exec("whoami")
			exec("device", "list")
		case "q", "quit", "exit":
			return nil
		default:
			fmt.Fprintln(out, "Please choose one of the listed options.")
		}
	}
}
