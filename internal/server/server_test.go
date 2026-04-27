package server

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/acidghost/k8s-crondash/internal/config"
	"github.com/acidghost/k8s-crondash/internal/k8s"
)

type mockService struct {
	jobs       []k8s.CronJobDisplay
	listErr    error
	triggerErr error
	ready      bool
}

func (m *mockService) ListCronJobs(_ context.Context) ([]k8s.CronJobDisplay, error) {
	return m.jobs, m.listErr
}

func (m *mockService) TriggerCronJob(_ context.Context, _, _ string) error {
	return m.triggerErr
}

func (m *mockService) IsReady() bool {
	return m.ready
}

func testConfig() *config.Config {
	return &config.Config{
		ListenAddr:      ":3000",
		Namespace:       "",
		RefreshInterval: 5,
		JobHistoryLimit: 5,
		AuthUsername:    "user",
		AuthPassword:    "pass",
	}
}

func basicAuthHeader(username, password string) string {
	credentials := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
}

func TestNew_Healthz_DoesNotRequireAuth(t *testing.T) {
	app := New(testConfig(), &mockService{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestNew_Readyz_ReflectsStoreReadiness(t *testing.T) {
	service := &mockService{ready: false}
	app := New(testConfig(), service)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when store is not ready, got %d", resp.StatusCode)
	}

	service.ready = true

	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when store is ready, got %d", resp.StatusCode)
	}
}

func TestNew_Dashboard_RequiresBasicAuth(t *testing.T) {
	app := New(testConfig(), &mockService{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestNew_Dashboard_AllowsValidBasicAuth(t *testing.T) {
	app := New(testConfig(), &mockService{
		jobs: []k8s.CronJobDisplay{{Name: "backup-job", Namespace: "default", Schedule: "0 2 * * *"}},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", basicAuthHeader("user", "pass"))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "backup-job") {
		t.Fatalf("expected dashboard response to contain cronjob name, got %q", string(body))
	}
}

func TestSHA256PasswordHash(t *testing.T) {
	got := sha256PasswordHash("pass")
	want := "{SHA256}10/w7o2juYBrGMh32/KbveULW9jk2tejpyUAD+uC6PE="

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
