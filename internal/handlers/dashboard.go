package handlers

import (
	"fmt"
	"net/http"

	"github.com/acidghost/k8s-crondash/internal/views"
	"github.com/gofiber/fiber/v3"
)

type DashboardHandler struct {
	service         CronJobService
	refreshInterval int
	showNamespace   bool
}

func NewDashboardHandler(service CronJobService, refreshInterval int, showNamespace bool) *DashboardHandler {
	return &DashboardHandler{
		service:         service,
		refreshInterval: refreshInterval,
		showNamespace:   showNamespace,
	}
}

func (h *DashboardHandler) Index(c fiber.Ctx) error {
	jobs, err := h.service.ListCronJobs(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("failed to load cronjobs")
	}
	_ = jobs
	return views.Render(c, views.Index())
}

func (h *DashboardHandler) CronJobs(c fiber.Ctx) error {
	jobs, err := h.service.ListCronJobs(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("failed to load cronjobs")
	}
	return c.Type("text").SendString(fmt.Sprintf("cronjob count: %d", len(jobs)))
}
