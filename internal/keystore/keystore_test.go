package keystore

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestSaveLoadDelete(t *testing.T) {
	keyring.MockInit()
	if _, err := Load("laptop"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("want ErrNoKey, got %v", err)
	}
	if err := Save("laptop", "AGE-SECRET-KEY-TEST"); err != nil {
		t.Fatal(err)
	}
	if got, err := Load("laptop"); err != nil || got != "AGE-SECRET-KEY-TEST" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := Load("workspace"); !errors.Is(err, ErrNoKey) {
		t.Fatal("devices must not share keys")
	}
	if err := Delete("laptop"); err != nil {
		t.Fatal(err)
	}
	if err := Delete("laptop"); err != nil {
		t.Fatal("deleting a missing key should not fail")
	}
}
