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
	defer pgConn.Close()

	pgIntrospector := postgres.NewIntrospector(pgConn, logger)
	pgApplier := postgres.NewApplier(pgConn, logger)
	flyReader := flyway.NewReader(logger)
	flyWriter := flyway.NewWriter(logger)
	projStore := projectfile.New(logger)
	cfgStore := config.New(logger)

	validator := core.NewValidator(logger)
	history := core.NewHistory(logger)
	drift := core.NewDriftDetector(logger)
	forward := core.NewForwardService(pgApplier, flyWriter, validator, logger)
	reverse := core.NewReverseService(pgIntrospector, flyReader, logger)
	project := core.NewProjectService(projStore, cfgStore, logger)

	application := ui.NewApp(forward, reverse, drift, project, validator, history, logger)
	if err := application.Run(); err != nil {
		logger.Error("application run", "err", err)
		os.Exit(1)
	}
}
