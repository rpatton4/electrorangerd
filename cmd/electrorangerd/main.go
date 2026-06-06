package main

import (
	"context"
	"log/slog"
	"os"

	pref "gioui.org/x/pref/theme"

	"github.com/rpatton4/electrorangerd/internal/adapter/config"
	"github.com/rpatton4/electrorangerd/internal/adapter/flyway"
	"github.com/rpatton4/electrorangerd/internal/adapter/postgres"
	"github.com/rpatton4/electrorangerd/internal/adapter/projectfile"
	ui "github.com/rpatton4/electrorangerd/internal/adapter/ui"
	"github.com/rpatton4/electrorangerd/internal/adapter/ui/theme"
	vaultadapter "github.com/rpatton4/electrorangerd/internal/adapter/vault"
	"github.com/rpatton4/electrorangerd/internal/core"
	"github.com/rpatton4/electrorangerd/internal/domain"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	pgConn, err := postgres.New(ctx, "", logger)
	if err != nil {
		logger.Error("postgres conn init", "err", err)
		os.Exit(1)
	}
	defer func() { _ = pgConn.Close() }()

	pgIntrospector := postgres.NewIntrospector(pgConn, logger)
	pgApplier := postgres.NewApplier(pgConn, logger)
	flyReader := flyway.NewReader(logger)
	flyWriter := flyway.NewWriter(logger)
	projStore := projectfile.New(logger)
	cfgStore := config.New(logger)
	cfg, err := cfgStore.Load()
	if err != nil {
		logger.Warn("config load failed, using defaults", "err", err)
		cfg = domain.Config{}
	}
	th := pickTheme(cfg)

	vaultStore, err := vaultadapter.NewStore(logger)
	if err != nil {
		logger.Error("vault store init", "err", err)
		os.Exit(1)
	}
	vaultKeychain := vaultadapter.NewKeychain(logger)
	vault, err := core.NewVault(ctx, vaultStore, vaultKeychain, logger)
	if err != nil {
		logger.Error("vault init", "err", err)
		os.Exit(1)
	}

	validator := core.NewValidator(logger)
	// Two history stacks: per-mode scoping per CLAUDE.md Plan C. Forward and
	// reverse engineering are one-shot operations and have no undo stack.
	diagramHistory := core.NewHistory(logger)
	dictionaryHistory := core.NewHistory(logger)
	drift := core.NewDriftDetector(logger)
	forward := core.NewForwardService(pgApplier, flyWriter, validator, logger)
	reverse := core.NewReverseService(pgIntrospector, flyReader, logger)
	// nil DictionaryStore: real on-disk persistence lands when adapter/dictionaryfile is added.
	project := core.NewProjectService(projStore, nil, cfgStore, logger)
	dictionarySvc := core.NewDictionaryService(nil, logger)
	diagramEditor := core.NewDiagramEditor(logger)

	application := ui.NewApp(forward, reverse, drift, project, validator, diagramHistory, dictionaryHistory, dictionarySvc, diagramEditor, vault, th, logger)
	if err := application.Run(); err != nil {
		logger.Error("application run", "err", err)
		os.Exit(1)
	}
}

// pickTheme resolves the active UI theme. An explicit ThemeName in config
// wins; otherwise we follow the OS dark/light preference, defaulting to
// Black and Chrome on unsupported platforms or when the API errors.
func pickTheme(cfg domain.Config) *theme.Theme {
	switch cfg.ThemeName {
	case "light":
		return theme.NewLightTheme()
	case "dark":
		return theme.NewDarkTheme()
	case "black-and-chrome":
		return theme.NewBlackAndChromeTheme()
	}
	if dark, err := pref.IsDarkMode(); err == nil && !dark {
		return theme.NewLightTheme()
	}
	return theme.NewBlackAndChromeTheme()
}
