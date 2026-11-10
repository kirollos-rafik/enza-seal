package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kirollos-rafik/enza-seal/internal/clipboard"
)

// fakeClipboard replaces the system clipboard for one test.
func fakeClipboard(t *testing.T, initial string) *string {
	t.Helper()
	store := initial
	oldR, oldW := clipboard.Read, clipboard.Write
	clipboard.Read = func() (string, error) { return store, nil }
	clipboard.Write = func(s string) error { store = s; return nil }
	t.Cleanup(func() { clipboard.Read, clipboard.Write = oldR, oldW })
	return &store
}

// setupAlice makes this "machine" ready: init + a team file listing alice.
func setupAlice(t *testing.T) string {
	t.Helper()
	device(t, "")
	if _, _, err := run(t, "", "init", "--person", "alice", "--name", "Alice", "--team", "QA"); err != nil {
		t.Fatal(err)
	}
	who, _, _ := run(t, "", "whoami")
	pub := publicKeyFromInit(t, who)
	t.Setenv("ENZA_SEAL_TEAM", writeTeam(t, map[string]map[string]string{"alice": {"laptop": pub}}, nil))
	return pub
}

func TestEncryptFromClipboardClearsPlaintext(t *testing.T) {
	setupAlice(t)
	clip := fakeClipboard(t, "4111 1111 1111 1111")
	out, _, err := run(t, "", "encrypt", "--to", "me", "--clip", "--no-copy")
	if err != nil {
		t.Fatal(err)
	}
	if *clip != "" {
		t.Fatalf("plaintext left on the clipboard: %q", *clip)
	}
	if strings.Contains(out, "4111") || !strings.Contains(out, armorBegin) {
		t.Fatalf("bad output %q", out)
	}
}

func TestEncryptCopiesSealedTextAndDecryptReadsClipboard(t *testing.T) {
	setupAlice(t)
	clip := fakeClipboard(t, "secret note")
	if _, _, err := run(t, "", "encrypt", "--to", "me", "--clip"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*clip, armorBegin) || strings.Contains(*clip, "secret note") {
		t.Fatalf("clipboard should hold the sealed block only, got %q", *clip)
	}
	// decrypt --copy: result goes to the clipboard, then is cleared.
	old := clearAfter
	clearAfter = 0
	defer func() { clearAfter = old }()
	sealed := *clip
	out, _, err := run(t, sealed, "decrypt", "--copy")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" || *clip != "" {
		t.Fatalf("decrypted text must not be printed or left on the clipboard (out=%q clip=%q)", out, *clip)
	}
}

func TestKeyBackupRestoreRoundTrip(t *testing.T) {
	oldWF := scryptWorkFactor
	scryptWorkFactor = 10 // fast for tests; real backups use the default
	defer func() { scryptWorkFactor = oldWF }()
	pub := setupAlice(t)
	backup := filepath.Join(t.TempDir(), "alice.key")
	if _, _, err := run(t, "correct horse\ncorrect horse\n", "key", "backup", "--out", backup); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(backup)
	if strings.Contains(string(b), "AGE-SECRET-KEY") {
		t.Fatal("backup is not encrypted")
	}

	// A fresh laptop: new keychain, new config, a different key.
	device(t, "")
	run(t, "", "init", "--person", "alice", "--name", "Alice", "--team", "QA")
	if _, _, err := run(t, "wrong passphrase\n", "key", "restore", "--file", backup); err == nil {
		t.Fatal("wrong passphrase accepted")
	}
	if _, _, err := run(t, "correct horse\n", "key", "restore", "--file", backup); err != nil {
		t.Fatal(err)
	}
	who, _, _ := run(t, "", "whoami")
	if got := publicKeyFromInit(t, who); got != pub {
		t.Fatalf("restored key differs: %s vs %s", got, pub)
	}
	if _, _, err := run(t, "short\nshort\n", "key", "backup", "--out", filepath.Join(t.TempDir(), "x"), "--force"); err == nil {
		t.Fatal("short passphrase accepted")
	}
}

func TestKeyRotateChangesPublicKey(t *testing.T) {
	old := setupAlice(t)
	if _, _, err := run(t, "", "key", "rotate"); err != nil {
		t.Fatal(err)
	}
	who, _, _ := run(t, "", "whoami")
	if publicKeyFromInit(t, who) == old {
		t.Fatal("key did not change")
	}
}

func TestEncryptOnlyDevice(t *testing.T) {
	pub := setupAlice(t)
	teamPath := os.Getenv("ENZA_SEAL_TEAM")

	// A different machine with no key of its own.
	device(t, teamPath)
	if _, _, err := run(t, "", "init", "--person", "alice", "--encrypt-only"); err != nil {
		t.Fatal(err)
	}
	sealed, _, err := run(t, "from a pool workspace", "encrypt", "--to", "me")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, sealed, "decrypt"); err == nil || !strings.Contains(err.Error(), "encrypt-only") {
		t.Fatalf("decrypt should explain encrypt-only, got %v", err)
	}
	_ = pub
	if _, _, err := run(t, "", "init", "--person", "stranger", "--encrypt-only", "--force"); err == nil {
		t.Fatal("encrypt-only must require a person already in the team file")
	}
}

func TestUninstallRemovesKeyAndSettings(t *testing.T) {
	setupAlice(t)
	home := os.Getenv("ENZA_SEAL_HOME")
	if _, _, err := run(t, "no\n", "uninstall"); err == nil {
		t.Fatal("must be cancelled without a yes")
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatal("cancelled uninstall removed files")
	}
	if _, _, err := run(t, "", "uninstall", "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); err == nil {
		t.Fatal("config folder still there")
	}
	if _, _, err := run(t, "x", "decrypt"); err == nil {
		t.Fatal("decrypt should fail after uninstall")
	}
}

func TestUpdateCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.0"}`))
	}))
	defer srv.Close()
	t.Setenv("ENZA_SEAL_RELEASE_API", srv.URL)
	out, _, err := run(t, "", "update")
	if err != nil || !strings.Contains(out, "latest is 1.2.0") || !strings.Contains(out, "install") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestMenuListsTeamAndQuits(t *testing.T) {
	setupAlice(t)
	out, errOut, err := run(t, "5\nbogus\nq\n", "menu")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "People:") || !strings.Contains(errOut, "Please choose one of the listed options") {
		t.Fatalf("menu output wrong:\nout=%q\nerr=%q", out, errOut)
	}
}
