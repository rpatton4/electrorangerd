package core

import (
	"log/slog"

	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/port"
)

type historyService struct {
	log       *slog.Logger
	stack     []domain.Command
	redoStack []domain.Command
}

// NewHistory returns a port.History backed by two in-memory stacks.
func NewHistory(log *slog.Logger) port.History {
	return &historyService{log: log}
}

func (s *historyService) Push(cmd domain.Command) {
	s.stack = append(s.stack, cmd)
	// A new command invalidates the redo stack.
	s.redoStack = s.redoStack[:0]
}

// Undo and Redo are stubs. Real invertibility requires either before/after
// project snapshots or per-command inverse operations; both are
// non-trivial design work in their own right, and a half-implementation
// (Push working, replay broken) is worse than honest stubs.
func (s *historyService) Undo() (domain.Command, bool) {
	return domain.Command{}, false
}

func (s *historyService) Redo() (domain.Command, bool) {
	return domain.Command{}, false
}
