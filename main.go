package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acidghost/k8s-crondash/internal/config"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
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

	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	app.Get("/readyz", func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	app.Use(basicauth.New(basicauth.Config{
		Authorizer: func(username, password string, _ fiber.Ctx) bool {
			return username == cfg.AuthUsername && password == cfg.AuthPassword
		},
		Realm: "Restricted",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("k8s-crondash dashboard")
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("listening", "addr", cfg.ListenAddr)

	if err := app.Listen(cfg.ListenAddr, fiber.ListenConfig{
		GracefulContext: ctx,
		ShutdownTimeout: 15 * time.Second,
	}); err != nil {
		slog.Error("server error", "error", err)
	}

	slog.Info("server stopped")
}
