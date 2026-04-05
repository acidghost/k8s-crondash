package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/acidghost/k8s-crondash/internal/k8s"
	"github.com/gofiber/fiber/v3"
)

type triggerMockService struct {
	jobs       []k8s.CronJobDisplay
	listFn     func() ([]k8s.CronJobDisplay, error)
	triggerErr error
}

func (m *triggerMockService) ListCronJobs(_ context.Context) ([]k8s.CronJobDisplay, error) {
	if m.listFn != nil {
		return m.listFn()
	}
	return m.jobs, nil
}

func (m *triggerMockService) TriggerCronJob(_ context.Context, _, _ string) error {
	return m.triggerErr
}

func setupTriggerApp(svc CronJobService) *fiber.App {
	h := NewTriggerHandler(svc)
	app := fiber.New()
	app.Get("/trigger-confirm/:ns/:name", h.ConfirmModal)
	app.Post("/trigger/:ns/:name", h.Trigger)
	return app
}

func TestConfirmModal_HTMX_ReturnsDialogHTML(t *testing.T) {
	svc := &triggerMockService{
		jobs: []k8s.CronJobDisplay{
			{Name: "my-cron", Namespace: "default", Schedule: "*/5 * * * *"},
		},
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/trigger-confirm/default/my-cron", nil)
	req.Header.Set("HX-Request", "true")
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

	if !strings.Contains(html, "<dialog") {
		t.Error("should contain dialog element")
	}
	if !strings.Contains(html, "my-cron") {
		t.Error("should contain cronjob name")
	}
	if !strings.Contains(html, "Trigger CronJob?") {
		t.Error("should contain confirm heading")
	}
	if !strings.Contains(html, "/trigger/default/my-cron") {
		t.Error("should contain trigger POST URL")
	}
}

func TestConfirmModal_NoJS_ReturnsFullPage(t *testing.T) {
	svc := &triggerMockService{
		jobs: []k8s.CronJobDisplay{
			{Name: "my-cron", Namespace: "default", Schedule: "*/5 * * * *"},
		},
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/trigger-confirm/default/my-cron", nil)
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

	if !strings.Contains(html, "<!doctype html>") {
		t.Error("should render full page with layout")
	}
	if !strings.Contains(html, "my-cron") {
		t.Error("should contain cronjob name")
	}
	if !strings.Contains(html, `<form method="POST" action="/trigger/default/my-cron"`) {
		t.Error("should have form with action for no-JS submit")
	}
	if strings.Contains(html, "<dialog") {
		t.Error("full page should NOT use dialog element")
	}
	if !strings.Contains(html, `<a href="/" class="<button>">Cancel</a>`) {
		t.Error("cancel should be styled link to home")
	}
}

func TestConfirmModal_RunningJob_ShowsWarning(t *testing.T) {
	svc := &triggerMockService{
		jobs: []k8s.CronJobDisplay{
			{Name: "running-cron", Namespace: "ns1", ActiveJobs: 2},
		},
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/trigger-confirm/ns1/running-cron", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "already running") {
		t.Error("should show running warning in modal")
	}
}

func TestConfirmModal_NotFound_Returns404(t *testing.T) {
	svc := &triggerMockService{
		jobs: []k8s.CronJobDisplay{},
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/trigger-confirm/default/nonexistent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestTrigger_HTMX_Success_ReturnsToast(t *testing.T) {
	svc := &triggerMockService{
		triggerErr: nil,
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/trigger/default/my-cron", nil)
	req.Header.Set("HX-Request", "true")
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

	if !strings.Contains(html, "Job triggered") {
		t.Error("should contain success toast message")
	}
	if !strings.Contains(html, "chip ok") {
		t.Error("success toast should have chip ok class")
	}
	if !strings.Contains(html, "toast-container") {
		t.Error("should contain toast container OOB swap")
	}
	if !strings.Contains(html, "modal-container") {
		t.Error("should clear modal container")
	}
	if !strings.Contains(html, "setTimeout") {
		t.Error("should contain auto-dismiss script")
	}
}

func TestTrigger_NoJS_Success_RedirectsWithFlash(t *testing.T) {
	svc := &triggerMockService{
		triggerErr: nil,
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/trigger/default/my-cron", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "flash=Job+triggered") {
		t.Errorf("redirect should contain flash param, got: %s", location)
	}
	if !strings.Contains(location, "flash-type=ok") {
		t.Errorf("redirect should contain flash-type=ok, got: %s", location)
	}
}

func TestTrigger_NoJS_Error_RedirectsWithErrorFlash(t *testing.T) {
	svc := &triggerMockService{
		triggerErr: fmt.Errorf("cronjob is suspended"),
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/trigger/default/my-cron", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "flash=") {
		t.Errorf("redirect should contain flash param, got: %s", location)
	}
	if !strings.Contains(location, "flash-type=bad") {
		t.Errorf("redirect should contain flash-type=bad, got: %s", location)
	}
}

func TestTrigger_HTMX_AlreadyRunning_ReturnsErrorToast(t *testing.T) {
	svc := &triggerMockService{
		triggerErr: fmt.Errorf("cronjob default/my-cron already has a running job: my-cron-123"),
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/trigger/default/my-cron", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "Trigger failed") {
		t.Error("should contain error toast prefix")
	}
	if !strings.Contains(html, "already has a running job") {
		t.Error("should contain original error message")
	}
	if !strings.Contains(html, "chip bad") {
		t.Error("error toast should have chip bad class")
	}
}

func TestTrigger_HTMX_Suspended_ReturnsErrorToast(t *testing.T) {
	svc := &triggerMockService{
		triggerErr: fmt.Errorf("cronjob default/suspended-cj is suspended"),
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/trigger/default/suspended-cj", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "suspended") {
		t.Error("should contain suspended error message")
	}
	if !strings.Contains(html, "chip bad") {
		t.Error("error toast should have chip bad class")
	}
}

func TestTrigger_HTMX_ServiceError_ReturnsErrorToast(t *testing.T) {
	svc := &triggerMockService{
		triggerErr: errors.New("k8s api timeout"),
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/trigger/default/my-cron", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "Trigger failed") {
		t.Error("should contain error toast prefix")
	}
	if !strings.Contains(html, "k8s api timeout") {
		t.Error("should contain error details")
	}
	if !strings.Contains(html, "chip bad") {
		t.Error("error toast should have chip bad class")
	}
}

func TestConfirmModal_ServiceError_Returns500(t *testing.T) {
	svc := &triggerMockService{
		listFn: func() ([]k8s.CronJobDisplay, error) {
			return nil, errors.New("k8s down")
		},
	}
	app := setupTriggerApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/trigger-confirm/default/my-cron", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}
