package core

import (
	"log/slog"

	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/port"
)

type validatorService struct {
	log *slog.Logger
}

// NewValidator constructs a Validator wired to a structured logger.
func NewValidator(log *slog.Logger) port.Validator {
	return &validatorService{log: log}
}

func (s *validatorService) Validate(_ domain.Project, _ domain.ValidationProfile) []domain.ValidationIssue {
	return nil
}
