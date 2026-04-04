package handlers

import (
	"net/http"

	"github.com/acidghost/k8s-crondash/internal/views"
	"github.com/gofiber/fiber/v3"
)

type DashboardHandler struct {
	service         CronJobService
	refreshInterval int
	showNamespace   bool
	namespace       string
}

func NewDashboardHandler(service CronJobService, refreshInterval int, showNamespace bool, namespace string) *DashboardHandler {
	return &DashboardHandler{
		service:         service,
		refreshInterval: refreshInterval,
		showNamespace:   showNamespace,
		namespace:       namespace,
	}
}

func (h *DashboardHandler) Index(c fiber.Ctx) error {
	jobs, err := h.service.ListCronJobs(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("failed to load cronjobs")
	}
	return views.Render(c, views.Dashboard(jobs, h.showNamespace, h.refreshInterval, h.namespace))
}

func (h *DashboardHandler) CronJobs(c fiber.Ctx) error {
	jobs, err := h.service.ListCronJobs(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("failed to load cronjobs")
	}
	return views.Render(c, views.CronJobTableBody(jobs, h.showNamespace))
}
