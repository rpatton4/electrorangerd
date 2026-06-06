package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/config"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/flyway"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/postgres"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/projectfile"
	ui "github.com/InfiniteSkye/electrorangerd/internal/adapter/ui"
	vaultadapter "github.com/InfiniteSkye/electrorangerd/internal/adapter/vault"
	"github.com/InfiniteSkye/electrorangerd/internal/core"
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
	history := core.NewHistory(logger)
	drift := core.NewDriftDetector(logger)
	forward := core.NewForwardService(pgApplier, flyWriter, validator, logger)
	reverse := core.NewReverseService(pgIntrospector, flyReader, logger)
	// nil DictionaryStore: real on-disk persistence lands when adapter/dictionaryfile is added.
	project := core.NewProjectService(projStore, nil, cfgStore, logger)
	dictionarySvc := core.NewDictionaryService(nil, logger)

	application := ui.NewApp(forward, reverse, drift, project, validator, history, dictionarySvc, vault, logger)
	if err := application.Run(); err != nil {
		logger.Error("application run", "err", err)
		os.Exit(1)
	}
}
