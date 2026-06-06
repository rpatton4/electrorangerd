//go:build !darwin && !windows && !linux

package vault

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/core"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
)

type keychainImpl struct {
	log *slog.Logger
}

// NewKeychain returns a core.Keychain that reports unavailable on platforms
// without an OS keychain integration. All operations return
// errs.ErrKeychainUnavailable.
func NewKeychain(log *slog.Logger) core.Keychain {
	return &keychainImpl{log: log}
}

func (k *keychainImpl) Available() bool { return false }

func (k *keychainImpl) Set(_ context.Context, _ []byte) error {
	return fmt.Errorf("keychain set: %w", errs.ErrKeychainUnavailable)
}

func (k *keychainImpl) Get(_ context.Context) ([]byte, error) {
	return nil, fmt.Errorf("keychain get: %w", errs.ErrKeychainUnavailable)
}

func (k *keychainImpl) Delete(_ context.Context) error {
	return fmt.Errorf("keychain delete: %w", errs.ErrKeychainUnavailable)
}
