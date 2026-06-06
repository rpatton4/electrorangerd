package core

import (
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

type validatorService struct {
	log *slog.Logger
}

// NewValidator returns a port.Validator. Validation is a pure in-memory
// operation, so no outbound adapters are needed.
func NewValidator(log *slog.Logger) port.Validator {
	return &validatorService{log: log}
}

func (s *validatorService) Validate(schema domain.Schema) []domain.ValidationIssue {
	return nil
}
