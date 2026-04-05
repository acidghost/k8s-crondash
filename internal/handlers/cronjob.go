package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/acidghost/k8s-crondash/internal/views"
	"github.com/gofiber/fiber/v3"
)

type TriggerHandler struct {
	service CronJobService
}

func NewTriggerHandler(service CronJobService) *TriggerHandler {
	return &TriggerHandler{service: service}
}

func (h *TriggerHandler) ConfirmModal(c fiber.Ctx) error {
	ns := c.Params("ns")
	name := c.Params("name")

	if ns == "" || name == "" {
		return c.Status(http.StatusBadRequest).SendString("missing namespace or name")
	}

	jobs, err := h.service.ListCronJobs(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("failed to load cronjobs")
	}

	for _, job := range jobs {
		if job.Namespace == ns && job.Name == name {
			return views.Render(c, views.TriggerConfirmModal(job))
		}
	}

	return c.Status(http.StatusNotFound).SendString("cronjob not found")
}

func (h *TriggerHandler) Trigger(c fiber.Ctx) error {
	ns := c.Params("ns")
	name := c.Params("name")

	if ns == "" || name == "" {
		return c.Status(http.StatusBadRequest).SendString("missing namespace or name")
	}

	err := h.service.TriggerCronJob(c.Context(), ns, name)
	if err != nil {
		slog.Error("trigger failed", "namespace", ns, "name", name, "error", err)
		return views.Render(c, views.Toast(fmt.Sprintf("Trigger failed: %s", err.Error()), false))
	}

	return views.Render(c, views.Toast("Job triggered", true))
}
