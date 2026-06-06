package config

import (
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/core"
	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
)

type store struct {
	log *slog.Logger
}

// New returns a core.ConfigStore that reads and writes user preferences to disk.
func New(log *slog.Logger) core.ConfigStore {
	return &store{log: log}
}

// Load reads the on-disk config file and returns the decoded Config.
func (s *store) Load() (domain.Config, error) {
	return domain.Config{}, fmt.Errorf("config load: %w", errs.ErrNotImplemented)
}

// Save encodes cfg and writes it to the on-disk config file.
func (s *store) Save(_ domain.Config) error {
	return fmt.Errorf("config save: %w", errs.ErrNotImplemented)
}
