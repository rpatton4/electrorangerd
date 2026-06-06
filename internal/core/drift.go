package core

import (
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

type driftService struct {
	log *slog.Logger
}

// NewDriftDetector returns a port.DriftDetector. Drift detection is a pure
// in-memory comparison, so no outbound adapters are needed.
func NewDriftDetector(log *slog.Logger) port.DriftDetector {
	return &driftService{log: log}
}

func (s *driftService) Compare(left, right domain.Schema) domain.DriftReport {
	return domain.DriftReport{}
}
