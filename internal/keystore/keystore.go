// Package keystore keeps the private key in the operating system's secure
// store: Windows Credential Manager, macOS Keychain, or Linux Secret Service.
package keystore

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const service = "enza-seal"

// ErrNoKey means no private key is stored for this device.
var ErrNoKey = errors.New("no private key found on this device: run enza-seal init")

func account(device string) string { return "device:" + device }

func Save(device, secret string) error {
	if err := keyring.Set(service, account(device), secret); err != nil {
		return fmt.Errorf("could not save the key in the OS secure store: %w", err)
	}
	return nil
}

func Load(device string) (string, error) {
	s, err := keyring.Get(service, account(device))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNoKey
	}
	if err != nil {
		return "", fmt.Errorf("could not read the key from the OS secure store: %w", err)
	}
	return s, nil
}

func Delete(device string) error {
	err := keyring.Delete(service, account(device))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
