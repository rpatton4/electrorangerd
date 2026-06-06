package core

import (
	"fmt"
	"log/slog"

	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/errs"
	"github.com/rpatton4/electrorangerd/internal/port"
)

// DictionaryStore loads and saves Dictionary documents from on-disk storage.
// Implemented by a future adapter/dictionaryfile when persistence lands.
// Declared here because dictionaryService is the primary consumer;
// projectService also references this same interface via package scope.
type DictionaryStore interface {
	Load(path string) (domain.Dictionary, error)
	Save(path string, dict domain.Dictionary) error
}

type dictionaryService struct {
	store DictionaryStore
	log   *slog.Logger
}

// NewDictionaryService constructs a DictionaryService wired to a Dictionary
// store and a structured logger. store may be nil during scaffolding; stub
// methods do not dereference it.
func NewDictionaryService(store DictionaryStore, log *slog.Logger) port.DictionaryService {
	return &dictionaryService{store: store, log: log}
}

func (s *dictionaryService) Get(_ domain.DictionaryRef) (domain.DictionaryEntry, bool) {
	return domain.DictionaryEntry{}, false
}

func (s *dictionaryService) Set(_ domain.DictionaryRef, _ domain.DictionaryEntry) error {
	return fmt.Errorf("dictionary set: %w", errs.ErrNotImplemented)
}
