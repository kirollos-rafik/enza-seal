package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// run executes the CLI in-process and returns stdout, stderr and the error.
func run(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := NewRoot("test")
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

// device simulates one machine: its own config folder and its own keychain.
func device(t *testing.T, teamFile string) {
	t.Helper()
	keyring.MockInit()
	t.Setenv("ENZA_SEAL_HOME", t.TempDir())
	t.Setenv("ENZA_SEAL_TEAM", teamFile)
	t.Setenv("ENZA_SEAL_NO_AUTOSYNC", "1")
}

func publicKeyFromInit(t *testing.T, whoami string) string {
	t.Helper()
	for _, l := range strings.Split(whoami, "\n") {
		if strings.HasPrefix(l, "Public key:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "Public key:"))
		}
	}
	t.Fatal("no public key in output")
	return ""
}

func writeTeam(t *testing.T, people map[string]map[string]string, groups map[string][]string) string {
	t.Helper()
	ps := map[string]any{}
	for id, devs := range people {
		ps[id] = map[string]any{"name": id, "devices": devs}
	}
	b, _ := json.Marshal(map[string]any{"version": 1, "people": ps, "groups": groups})
	p := filepath.Join(t.TempDir(), "recipients.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInitEncryptDecryptRoundTrip(t *testing.T) {
	device(t, "")
	if _, _, err := run(t, "", "init", "--person", "alice", "--name", "Alice", "--team", "QA"); err != nil {
		t.Fatal(err)
	}
	who, _, err := run(t, "", "whoami")
	if err != nil {
		t.Fatal(err)
	}
	pub := publicKeyFromInit(t, who)

	// Team file now lists alice's device, so she can encrypt to herself.
	t.Setenv("ENZA_SEAL_TEAM", writeTeam(t, map[string]map[string]string{"alice": {"laptop": pub}}, nil))

	sealed, _, err := run(t, "4111 1111 1111 1111", "encrypt", "--to", "me")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sealed, "-----BEGIN AGE ENCRYPTED FILE-----") || strings.Contains(sealed, "4111") {
		t.Fatalf("unexpected sealed output: %q", sealed)
	}
	plain, _, err := run(t, sealed, "decrypt")
	if err != nil || plain != "4111 1111 1111 1111" {
		t.Fatalf("got %q, %v", plain, err)
	}
}

func TestFileRoundTripAndNoOverwrite(t *testing.T) {
	device(t, "")
	if _, _, err := run(t, "", "init", "--person", "alice", "--name", "Alice", "--team", "QA"); err != nil {
		t.Fatal(err)
	}
	who, _, _ := run(t, "", "whoami")
	t.Setenv("ENZA_SEAL_TEAM", writeTeam(t, map[string]map[string]string{"alice": {"laptop": publicKeyFromInit(t, who)}}, nil))

	dir := t.TempDir()
	src := filepath.Join(dir, "cards.csv")
	if err := os.WriteFile(src, []byte("pan,exp\n4111111111111111,12/29\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, "", "encrypt", "--to", "me", "-f", src); err != nil {
		t.Fatal(err)
	}
	sealed, _ := os.ReadFile(src + ".seal")
	if bytes.Contains(sealed, []byte("4111")) {
		t.Fatal("plaintext in .seal file")
	}
	if _, _, err := run(t, "", "decrypt", "-f", src+".seal"); err == nil {
		t.Fatal("decrypt must refuse to overwrite the existing original")
	}
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, "", "decrypt", "-f", src+".seal"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(src)
	if string(got) != "pan,exp\n4111111111111111,12/29\n" {
		t.Fatalf("got %q", got)
	}
}

func TestTamperedFileLeavesNoOutput(t *testing.T) {
	device(t, "")
	run(t, "", "init", "--person", "alice", "--name", "Alice", "--team", "QA")
	who, _, _ := run(t, "", "whoami")
	t.Setenv("ENZA_SEAL_TEAM", writeTeam(t, map[string]map[string]string{"alice": {"laptop": publicKeyFromInit(t, who)}}, nil))

	dir := t.TempDir()
	src := filepath.Join(dir, "data.bin")
	_ = os.WriteFile(src, bytes.Repeat([]byte("x"), 200000), 0o600)
	if _, _, err := run(t, "", "encrypt", "--to", "me", "-f", src); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(src + ".seal")
	b[len(b)-3] ^= 0xff
	_ = os.WriteFile(src+".seal", b, 0o600)
	_ = os.Remove(src)
	if _, _, err := run(t, "", "decrypt", "-f", src+".seal"); err == nil {
		t.Fatal("tampered file accepted")
	}
	if _, err := os.Stat(src); err == nil {
		t.Fatal("partial output left behind")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".enza-seal-*")); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestOnlyNamedRecipientCanDecrypt(t *testing.T) {
	// Bob's machine: create his key, publish his public key.
	device(t, "")
	run(t, "", "init", "--person", "bob", "--name", "Bob", "--team", "Pay")
	bobWho, _, _ := run(t, "", "whoami")
	bobPub := publicKeyFromInit(t, bobWho)
	bobHome := os.Getenv("ENZA_SEAL_HOME")

	// Carol's machine (own keychain).
	device(t, "")
	run(t, "", "init", "--person", "carol", "--name", "Carol", "--team", "Pay")
	carolWho, _, _ := run(t, "", "whoami")
	carolPub := publicKeyFromInit(t, carolWho)
	carolHome := os.Getenv("ENZA_SEAL_HOME")

	teamPath := writeTeam(t, map[string]map[string]string{"bob": {"laptop": bobPub}, "carol": {"laptop": carolPub}}, map[string][]string{"pay": {"bob", "carol"}})
	t.Setenv("ENZA_SEAL_TEAM", teamPath)

	// Sealed for Bob only.
	sealedBob, _, err := run(t, "for bob", "encrypt", "--to", "bob")
	if err != nil {
		t.Fatal(err)
	}
	// Carol cannot open it.
	if _, _, err := run(t, sealedBob, "decrypt"); err == nil || !strings.Contains(err.Error(), "not encrypted for your key") {
		t.Fatalf("carol should be rejected, got %v", err)
	}
	// Sealed for the group: Carol can open it.
	sealedGroup, _, _ := run(t, "for all", "encrypt", "--to", "@pay")
	if got, _, err := run(t, sealedGroup, "decrypt"); err != nil || got != "for all" {
		t.Fatalf("group message: %q, %v", got, err)
	}
	_, _ = bobHome, carolHome
}

func TestInitRefusesToOverwriteWithoutForce(t *testing.T) {
	device(t, "")
	if _, _, err := run(t, "", "init", "--person", "alice", "--name", "A", "--team", "T"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, "", "init", "--person", "alice", "--name", "A", "--team", "T"); err == nil {
		t.Fatal("second init should be refused")
	}
	if _, _, err := run(t, "", "init", "--person", "alice", "--name", "A", "--team", "T", "--force"); err != nil {
		t.Fatalf("--force should work: %v", err)
	}
}

func TestEncryptNeedsRecipientAndTeam(t *testing.T) {
	device(t, filepath.Join(t.TempDir(), "missing.json"))
	if _, _, err := run(t, "x", "encrypt"); err == nil {
		t.Fatal("encrypt without --to must fail")
	}
	if _, _, err := run(t, "x", "encrypt", "--to", "bob"); err == nil {
		t.Fatal("encrypt without a team file must fail")
	}
}
