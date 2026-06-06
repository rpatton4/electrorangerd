package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/InfiniteSkye/electrorangerd/internal/core"
	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
)

const (
	configSubdir = "electrorangerd"
	vaultFile    = "vault.json"
)

type fileStore struct {
	path string
	log  *slog.Logger
}

// NewStore constructs a core.EncryptedStore that persists the vault blob to
// <UserConfigDir>/electrorangerd/vault.json. The directory is created on
// first save with 0700 permissions; the vault file is written 0600.
func NewStore(log *slog.Logger) (core.EncryptedStore, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("vault store: locate config dir: %w", err)
	}
	return &fileStore{
		path: filepath.Join(cfg, configSubdir, vaultFile),
		log:  log,
	}, nil
}

// NewStoreAtPath constructs a core.EncryptedStore that persists to an explicit
// path. Useful for testing and for hosting the vault in a non-default
// location.
func NewStoreAtPath(path string, log *slog.Logger) core.EncryptedStore {
	return &fileStore{path: path, log: log}
}

func (s *fileStore) Load(_ context.Context) (domain.VaultBlob, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.VaultBlob{}, errs.ErrNotFound
	}
	if err != nil {
		return domain.VaultBlob{}, fmt.Errorf("vault store: read %s: %w", s.path, err)
	}
	var blob domain.VaultBlob
	if err := json.Unmarshal(data, &blob); err != nil {
		return domain.VaultBlob{}, fmt.Errorf("vault store: parse %s: %w", s.path, errs.ErrSerialization)
	}
	return blob, nil
}

func (s *fileStore) Save(_ context.Context, blob domain.VaultBlob) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("vault store: mkdir %s: %w", filepath.Dir(s.path), err)
	}
	data, err := json.MarshalIndent(blob, "", "  ")
	if err != nil {
		return fmt.Errorf("vault store: marshal: %w", errs.ErrSerialization)
	}
	// Atomic write: write to a temp file in the same directory, then rename.
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "vault-*.tmp")
	if err != nil {
		return fmt.Errorf("vault store: tempfile: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("vault store: write tempfile: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("vault store: chmod tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("vault store: close tempfile: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("vault store: rename: %w", err)
	}
	return nil
}

func (s *fileStore) Delete(_ context.Context) error {
	if err := os.Remove(s.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("vault store: remove %s: %w", s.path, err)
	}
	return nil
}
