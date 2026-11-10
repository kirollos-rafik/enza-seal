package crypto

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"filippo.io/age"
)

func newID(t *testing.T) *age.X25519Identity {
	t.Helper()
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func seal(t *testing.T, plain string, armored bool, ids ...*age.X25519Identity) []byte {
	t.Helper()
	var keys []string
	for _, id := range ids {
		keys = append(keys, id.Recipient().String())
	}
	rcpts, err := ParseRecipients(keys)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Encrypt(&out, strings.NewReader(plain), rcpts, armored); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestRoundTripArmoredAndBinary(t *testing.T) {
	id := newID(t)
	for _, armored := range []bool{true, false} {
		ct := seal(t, "4111 1111 1111 1111", armored, id)
		if armored && !bytes.HasPrefix(ct, []byte(armorHeader)) {
			t.Fatal("expected armored output")
		}
		if bytes.Contains(ct, []byte("4111")) {
			t.Fatal("plaintext leaked into ciphertext")
		}
		got, err := DecryptAll(bytes.NewReader(ct), id)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "4111 1111 1111 1111" {
			t.Fatalf("got %q", got)
		}
	}
}

func TestEveryRecipientCanDecrypt(t *testing.T) {
	laptop, workspace := newID(t), newID(t)
	ct := seal(t, "hello", true, laptop, workspace)
	for _, id := range []*age.X25519Identity{laptop, workspace} {
		got, err := DecryptAll(bytes.NewReader(ct), id)
		if err != nil || string(got) != "hello" {
			t.Fatalf("got %q, %v", got, err)
		}
	}
}

func TestWrongKeyIsRejected(t *testing.T) {
	ct := seal(t, "secret", true, newID(t))
	_, err := DecryptAll(bytes.NewReader(ct), newID(t))
	if !errors.Is(err, ErrNotRecipient) {
		t.Fatalf("want ErrNotRecipient, got %v", err)
	}
}

func TestTamperedDataIsRejected(t *testing.T) {
	id := newID(t)
	ct := seal(t, strings.Repeat("x", 1000), false, id)
	ct[len(ct)-5] ^= 0xff
	if _, err := DecryptAll(bytes.NewReader(ct), id); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}

func TestLeadingWhitespaceFromPasteIsIgnored(t *testing.T) {
	id := newID(t)
	ct := append([]byte("\n  \r\n"), seal(t, "pasted", true, id)...)
	got, err := DecryptAll(bytes.NewReader(ct), id)
	if err != nil || string(got) != "pasted" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestEmptyPlaintext(t *testing.T) {
	id := newID(t)
	got, err := DecryptAll(bytes.NewReader(seal(t, "", true, id)), id)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestParseRecipientsRejectsBadInput(t *testing.T) {
	if _, err := ParseRecipients(nil); err == nil {
		t.Fatal("empty list accepted")
	}
	if _, err := ParseRecipients([]string{"not-a-key"}); err == nil {
		t.Fatal("bad key accepted")
	}
}

func TestFingerprintIsStableAndShort(t *testing.T) {
	k := newID(t).Recipient().String()
	a, b := Fingerprint(k), Fingerprint(k)
	if a != b || len(a) != 9 || a[4] != '-' {
		t.Fatalf("bad fingerprint %q", a)
	}
}
