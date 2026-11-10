// Package crypto is a thin wrapper over filippo.io/age. It adds no
// cryptography of its own.
package crypto

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

const armorHeader = "-----BEGIN AGE ENCRYPTED FILE-----"

// ErrNotRecipient means the data was not encrypted for the key we hold.
var ErrNotRecipient = errors.New("this message was not encrypted for your key")

// GenerateIdentity creates a new X25519 key pair.
func GenerateIdentity() (*age.X25519Identity, error) {
	return age.GenerateX25519Identity()
}

// ParseRecipients turns public key strings into age recipients.
func ParseRecipients(keys []string) ([]age.Recipient, error) {
	if len(keys) == 0 {
		return nil, errors.New("no recipients given")
	}
	out := make([]age.Recipient, 0, len(keys))
	for _, k := range keys {
		r, err := age.ParseX25519Recipient(strings.TrimSpace(k))
		if err != nil {
			return nil, fmt.Errorf("invalid public key %q: %w", k, err)
		}
		out = append(out, r)
	}
	return out, nil
}

// Encrypt reads src and writes the sealed result to dst, optionally as
// ASCII armor (safe to paste into chat, email and tickets).
func Encrypt(dst io.Writer, src io.Reader, rcpts []age.Recipient, armored bool) error {
	var aw io.WriteCloser
	w := dst
	if armored {
		aw = armor.NewWriter(dst)
		w = aw
	}
	ew, err := age.Encrypt(w, rcpts...)
	if err != nil {
		return err
	}
	if _, err := io.Copy(ew, src); err != nil {
		return err
	}
	if err := ew.Close(); err != nil {
		return err
	}
	if aw != nil {
		return aw.Close()
	}
	return nil
}

// Decrypt returns a reader over the plaintext. Armored and binary input are
// both accepted. Callers that write to disk must buffer or use a temp file,
// because age reports tampering only when it reaches the damaged chunk.
func Decrypt(src io.Reader, id age.Identity) (io.Reader, error) {
	br := bufio.NewReader(src)
	for {
		b, err := br.Peek(1)
		if err != nil || !isSpace(b[0]) {
			break
		}
		_, _ = br.Discard(1)
	}
	var r io.Reader = br
	if head, _ := br.Peek(len(armorHeader)); string(head) == armorHeader {
		r = armor.NewReader(br)
	}
	pr, err := age.Decrypt(r, id)
	if err != nil {
		var nm *age.NoIdentityMatchError
		if errors.As(err, &nm) {
			return nil, ErrNotRecipient
		}
		return nil, err
	}
	return pr, nil
}

// DecryptAll decrypts everything into memory.
func DecryptAll(src io.Reader, id age.Identity) ([]byte, error) {
	pr, err := Decrypt(src, id)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, pr); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Fingerprint is a short code derived from a public key, e.g. "QZ7V-K3XP",
// for confirming a key by voice or in person.
func Fingerprint(publicKey string) string {
	sum := sha256.Sum256([]byte(publicKey))
	s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])[:8]
	return s[:4] + "-" + s[4:]
}

func isSpace(b byte) bool { return b == ' ' || b == '\n' || b == '\r' || b == '\t' }
