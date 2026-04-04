package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/acidghost/k8s-crondash/internal/k8s"
	"github.com/gofiber/fiber/v3"
)

type mockService struct {
	jobs []k8s.CronJobDisplay
	err  error
}

func (m *mockService) ListCronJobs(_ context.Context) ([]k8s.CronJobDisplay, error) {
	return m.jobs, m.err
}

func setupApp(svc CronJobService) *fiber.App {
	h := NewDashboardHandler(svc, 5, true, "default")
	app := fiber.New()
	app.Get("/", h.Index)
	app.Get("/cronjobs", h.CronJobs)
	return app
}

func TestIndex_Returns200WithTable(t *testing.T) {
	now := time.Now()
	svc := &mockService{
		jobs: []k8s.CronJobDisplay{
			{Name: "my-cron", Namespace: "default", Schedule: "*/5 * * * *", Running: true, ActiveJobs: 1, LastSuccess: &now},
		},
	}
	app := setupApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "my-cron") {
		t.Error("response should contain cronjob name")
	}
	if !strings.Contains(html, "*/5 * * * *") {
		t.Error("response should contain schedule")
	}
	if !strings.Contains(html, "<table") {
		t.Error("response should contain table element")
	}
	if !strings.Contains(html, "Running") {
		t.Error("response should show Running badge")
	}
}

func TestIndex_EmptyData_ShowsEmptyState(t *testing.T) {
	svc := &mockService{jobs: []k8s.CronJobDisplay{}}
	app := setupApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "No CronJobs found") {
		t.Error("response should contain empty state message")
	}
}

func TestCronJobs_ReturnsTableBody(t *testing.T) {
	svc := &mockService{
		jobs: []k8s.CronJobDisplay{
			{Name: "job-a", Namespace: "ns1", Schedule: "0 * * * *"},
			{Name: "job-b", Namespace: "ns2", Schedule: "*/10 * * * *", Suspended: true},
		},
	}
	app := setupApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/cronjobs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "<tr") {
		t.Error("response should contain table rows")
	}
	if !strings.Contains(html, "job-a") {
		t.Error("response should contain job-a")
	}
	if !strings.Contains(html, "job-b") {
		t.Error("response should contain job-b")
	}
	if !strings.Contains(html, "Suspended") {
		t.Error("response should contain Suspended badge")
	}
}

func TestCronJobs_EmptyData(t *testing.T) {
	svc := &mockService{jobs: []k8s.CronJobDisplay{}}
	app := setupApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/cronjobs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "No CronJobs found") {
		t.Error("empty cronjobs should show no data message")
	}
}

func TestIndex_ServiceError_Returns500(t *testing.T) {
	svc := &mockService{err: errors.New("k8s down")}
	app := setupApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestCronJobs_ServiceError_Returns500(t *testing.T) {
	svc := &mockService{err: errors.New("k8s down")}
	app := setupApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/cronjobs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}
