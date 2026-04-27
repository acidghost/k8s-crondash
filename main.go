package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acidghost/k8s-crondash/internal/config"
	"github.com/acidghost/k8s-crondash/internal/k8s"
	"github.com/acidghost/k8s-crondash/internal/server"
	"github.com/acidghost/k8s-crondash/internal/state"
	"github.com/gofiber/fiber/v3"
)

var (
	buildVersion string
	buildCommit  string
	buildDate    string
)

func main() {
	slog.Info("starting k8s-crondash",
		"version", buildVersion,
		"commit", buildCommit,
		"date", buildDate,
	)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	clientset, err := k8s.NewClientSet("")
	if err != nil {
		slog.Error("failed to create Kubernetes client", "error", err)
		os.Exit(1)
	}

	store := state.NewStore(ctx, clientset, cfg.Namespace, time.Duration(cfg.RefreshInterval)*time.Second, cfg.JobHistoryLimit)
	app := server.New(cfg, store)
	cfg.AuthPassword = ""

	slog.Info("listening", "addr", cfg.ListenAddr)

	if err := app.Listen(cfg.ListenAddr, fiber.ListenConfig{
		GracefulContext: ctx,
		ShutdownTimeout: 15 * time.Second,
	}); err != nil {
		slog.Error("server error", "error", err)
	}

	slog.Info("server stopped")
}
