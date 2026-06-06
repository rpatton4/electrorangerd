package core

import (
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

type driftService struct {
	log *slog.Logger
}

// NewDriftDetector constructs a DriftDetector wired to a structured logger.
func NewDriftDetector(log *slog.Logger) port.DriftDetector {
	return &driftService{log: log}
}

func (s *driftService) Compare(_, _ domain.Project) domain.DriftReport {
	return domain.DriftReport{}
}
