package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

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
			if views.IsHTMX(c) {
				return views.Render(c, views.TriggerConfirmModal(job))
			}
			return views.Render(c, views.TriggerConfirmPage(job))
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
		if views.IsHTMX(c) {
			return views.Render(c, views.Toast(fmt.Sprintf("Trigger failed: %s", err.Error()), false))
		}
		flash := url.QueryEscape(fmt.Sprintf("Trigger failed: %s", err.Error()))
		return c.Redirect().Status(http.StatusSeeOther).To("/?flash=" + flash + "&flash-type=bad")
	}

	if views.IsHTMX(c) {
		return views.Render(c, views.Toast("Job triggered", true))
	}
	return c.Redirect().Status(http.StatusSeeOther).To("/?flash=Job+triggered&flash-type=ok")
}
