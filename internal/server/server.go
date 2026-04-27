package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"

	"github.com/acidghost/k8s-crondash/internal/config"
	"github.com/acidghost/k8s-crondash/internal/handlers"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

type Service interface {
	handlers.CronJobService
	IsReady() bool
}

func New(cfg *config.Config, service Service) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "k8s-crondash",
	})

	app.Use(logger.New())

	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	app.Get("/readyz", func(c fiber.Ctx) error {
		if service.IsReady() {
			return c.SendStatus(http.StatusOK)
		}
		return c.SendStatus(http.StatusServiceUnavailable)
	})

	app.Use(basicauth.New(basicauth.Config{
		Users: map[string]string{
			cfg.AuthUsername: sha256PasswordHash(cfg.AuthPassword),
		},
	}))

	dashboardHandler := handlers.NewDashboardHandler(service, cfg.RefreshInterval, cfg.Namespace == "", cfg.Namespace)
	app.Get("/", dashboardHandler.Index)
	app.Get("/cronjobs", dashboardHandler.CronJobs)

	triggerHandler := handlers.NewTriggerHandler(service)
	app.Get("/trigger-confirm/:ns/:name", triggerHandler.ConfirmModal)
	app.Post("/trigger/:ns/:name", triggerHandler.Trigger)

	return app
}

func sha256PasswordHash(password string) string {
	h := sha256.Sum256([]byte(password))
	return "{SHA256}" + base64.StdEncoding.EncodeToString(h[:])
}
