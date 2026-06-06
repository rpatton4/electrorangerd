//go:build darwin || windows || linux

package vault

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	"github.com/zalando/go-keyring"

	"github.com/InfiniteSkye/electrorangerd/internal/core"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
)

const (
	keychainService = "ElectroRangerD"
	keychainAccount = "vault-dek"
)

type keychainImpl struct {
	log *slog.Logger
}

// NewKeychain constructs a core.Keychain backed by the OS keychain (macOS
// Keychain on darwin, Windows Credential Manager on windows, Secret Service
// via D-Bus on linux). The DEK is stored base64-encoded under the
// "ElectroRangerD" service and "vault-dek" account.
func NewKeychain(log *slog.Logger) core.Keychain {
	return &keychainImpl{log: log}
}

func (k *keychainImpl) Available() bool {
	return true
}

func (k *keychainImpl) Set(_ context.Context, dek []byte) error {
	if err := keyring.Set(keychainService, keychainAccount, base64.StdEncoding.EncodeToString(dek)); err != nil {
		return fmt.Errorf("keychain set: %w", err)
	}
	return nil
}

func (k *keychainImpl) Get(_ context.Context) ([]byte, error) {
	s, err := keyring.Get(keychainService, keychainAccount)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("keychain get: %w", err)
	}
	dek, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("keychain decode: %w", errs.ErrSerialization)
	}
	return dek, nil
}

func (k *keychainImpl) Delete(_ context.Context) error {
	err := keyring.Delete(keychainService, keychainAccount)
	if errors.Is(err, keyring.ErrNotFound) {
		return errs.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("keychain delete: %w", err)
	}
	return nil
}
