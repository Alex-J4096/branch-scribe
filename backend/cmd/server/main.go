package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"branchscribe/backend/internal/api"
	"branchscribe/backend/internal/block"
	"branchscribe/backend/internal/branch"
	"branchscribe/backend/internal/config"
	"branchscribe/backend/internal/database"
	"branchscribe/backend/internal/graph"
	"branchscribe/backend/internal/project"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	router := api.NewRouter(cfg, db)
	apiGroup := router.Group("/api")
	project.RegisterRoutes(apiGroup, project.NewHandler(project.NewRepository(db)))
	branch.RegisterRoutes(apiGroup, branch.NewHandler(branch.NewRepository(db)))
	block.RegisterRoutes(apiGroup, block.NewHandler(block.NewRepository(db)))
	graph.RegisterRoutes(apiGroup, graph.NewHandler(graph.NewRepository(db)))
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("starting backend", "addr", cfg.HTTPAddr, "env", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("shutdown server", "error", err)
		os.Exit(1)
	}
}
