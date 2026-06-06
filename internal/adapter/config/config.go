package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/rpatton4/electrorangerd/internal/core"
	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/errs"
)

const (
	configSubdir = "electrorangerd"
	configFile   = "config.json"
)

type store struct {
	path string
	log  *slog.Logger
}

// New returns a core.ConfigStore that reads and writes user preferences to
// <UserConfigDir>/electrorangerd/config.json. Mirrors the vault adapter's
// atomic-rename write pattern; the directory is created on first save with
// 0700 permissions and the config file is written 0600.
func New(log *slog.Logger) core.ConfigStore {
	cfg, err := os.UserConfigDir()
	if err != nil {
		log.Warn("config store: locate user config dir failed; persistence disabled", "err", err)
		return &store{log: log}
	}
	return &store{
		path: filepath.Join(cfg, configSubdir, configFile),
		log:  log,
	}
}

// Load reads the on-disk config file. A missing file is normal on first
// launch and returns a zero Config with no error.
func (s *store) Load() (domain.Config, error) {
	if s.path == "" {
		return domain.Config{}, nil
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.Config{}, nil
	}
	if err != nil {
		return domain.Config{}, fmt.Errorf("config load: read %s: %w", s.path, err)
	}
	var cfg domain.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return domain.Config{}, fmt.Errorf("config load: parse %s: %w", s.path, errs.ErrSerialization)
	}
	return cfg, nil
}

// Save encodes cfg and writes it atomically to the config file.
func (s *store) Save(cfg domain.Config) error {
	if s.path == "" {
		return fmt.Errorf("config save: no user config dir available")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("config save: mkdir %s: %w", filepath.Dir(s.path), err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config save: marshal: %w", errs.ErrSerialization)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "config-*.tmp")
	if err != nil {
		return fmt.Errorf("config save: tempfile: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("config save: write tempfile: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("config save: chmod tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("config save: close tempfile: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("config save: rename: %w", err)
	}
	return nil
}
