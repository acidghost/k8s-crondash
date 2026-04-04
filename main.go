package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acidghost/k8s-crondash/internal/config"
	"github.com/acidghost/k8s-crondash/internal/k8s"
	"github.com/acidghost/k8s-crondash/internal/state"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/gofiber/fiber/v3/middleware/logger"
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

	app := fiber.New(fiber.Config{
		AppName: "k8s-crondash",
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	clientset, err := k8s.NewClientSet("")
	if err != nil {
		slog.Error("failed to create Kubernetes client", "error", err)
		os.Exit(1)
	}

	store := state.NewStore(ctx, clientset, cfg.Namespace, time.Duration(cfg.RefreshInterval)*time.Second, cfg.JobHistoryLimit)

	app.Use(logger.New())

	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	app.Get("/readyz", func(c fiber.Ctx) error {
		if store.IsReady() {
			return c.SendStatus(http.StatusOK)
		}
		return c.SendStatus(http.StatusServiceUnavailable)
	})

	app.Use(basicauth.New(basicauth.Config{
		Users: map[string]string{
			cfg.AuthUsername: sha256PasswordHash(cfg.AuthPassword),
		},
	}))

	cfg.AuthPassword = ""

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("k8s-crondash dashboard")
	})

	slog.Info("listening", "addr", cfg.ListenAddr)

	if err := app.Listen(cfg.ListenAddr, fiber.ListenConfig{
		GracefulContext: ctx,
		ShutdownTimeout: 15 * time.Second,
	}); err != nil {
		slog.Error("server error", "error", err)
	}

	slog.Info("server stopped")
}

func sha256PasswordHash(password string) string {
	h := sha256.Sum256([]byte(password))
	return "{SHA256}" + base64.StdEncoding.EncodeToString(h[:])
}
