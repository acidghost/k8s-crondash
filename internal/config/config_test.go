package config

import (
	"os"
	"strings"
	"testing"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		"CRONDASH_LISTEN_ADDR",
		"CRONDASH_NAMESPACE",
		"CRONDASH_REFRESH_INTERVAL",
		"CRONDASH_JOB_HISTORY_LIMIT",
		"CRONDASH_AUTH_USERNAME",
		"CRONDASH_AUTH_PASSWORD",
	}

	previous := make(map[string]*string, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			v := value
			previous[key] = &v
		} else {
			previous[key] = nil
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}

	t.Cleanup(func() {
		for _, key := range keys {
			if previous[key] == nil {
				if err := os.Unsetenv(key); err != nil {
					t.Fatalf("restore unset %s: %v", key, err)
				}
				continue
			}
			if err := os.Setenv(key, *previous[key]); err != nil {
				t.Fatalf("restore %s: %v", key, err)
			}
		}
	})
}

func withArgs(t *testing.T, args ...string) {
	t.Helper()

	previous := os.Args
	os.Args = args
	t.Cleanup(func() {
		os.Args = previous
	})
}

func TestLoad_FromFlags(t *testing.T) {
	clearConfigEnv(t)
	withArgs(t,
		"k8s-crondash",
		"--listen-addr=:4000",
		"--namespace=prod",
		"--refresh-interval=10",
		"--job-history-limit=7",
		"--auth-username=user",
		"--auth-password=pass",
	)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != ":4000" {
		t.Fatalf("expected listen addr :4000, got %q", cfg.ListenAddr)
	}
	if cfg.Namespace != "prod" {
		t.Fatalf("expected namespace prod, got %q", cfg.Namespace)
	}
	if cfg.RefreshInterval != 10 {
		t.Fatalf("expected refresh interval 10, got %d", cfg.RefreshInterval)
	}
	if cfg.JobHistoryLimit != 7 {
		t.Fatalf("expected job history limit 7, got %d", cfg.JobHistoryLimit)
	}
	if cfg.AuthUsername != "user" {
		t.Fatalf("expected auth username user, got %q", cfg.AuthUsername)
	}
	if cfg.AuthPassword != "pass" {
		t.Fatalf("expected auth password pass, got %q", cfg.AuthPassword)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	clearConfigEnv(t)
	withArgs(t, "k8s-crondash")

	t.Setenv("CRONDASH_LISTEN_ADDR", ":5000")
	t.Setenv("CRONDASH_NAMESPACE", "staging")
	t.Setenv("CRONDASH_REFRESH_INTERVAL", "11")
	t.Setenv("CRONDASH_JOB_HISTORY_LIMIT", "9")
	t.Setenv("CRONDASH_AUTH_USERNAME", "env-user")
	t.Setenv("CRONDASH_AUTH_PASSWORD", "env-pass")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != ":5000" {
		t.Fatalf("expected listen addr :5000, got %q", cfg.ListenAddr)
	}
	if cfg.Namespace != "staging" {
		t.Fatalf("expected namespace staging, got %q", cfg.Namespace)
	}
	if cfg.RefreshInterval != 11 {
		t.Fatalf("expected refresh interval 11, got %d", cfg.RefreshInterval)
	}
	if cfg.JobHistoryLimit != 9 {
		t.Fatalf("expected job history limit 9, got %d", cfg.JobHistoryLimit)
	}
	if cfg.AuthUsername != "env-user" {
		t.Fatalf("expected auth username env-user, got %q", cfg.AuthUsername)
	}
	if cfg.AuthPassword != "env-pass" {
		t.Fatalf("expected auth password env-pass, got %q", cfg.AuthPassword)
	}
}

func TestLoad_InvalidValues(t *testing.T) {
	clearConfigEnv(t)
	withArgs(t,
		"k8s-crondash",
		"--listen-addr=not-an-addr",
		"--refresh-interval=0",
		"--job-history-limit=0",
		"--auth-username=user",
		"--auth-password=pass",
	)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}

	message := err.Error()
	if !strings.Contains(message, `listen-addr "not-an-addr" is not a valid host:port`) {
		t.Fatalf("expected listen-addr validation error, got %q", message)
	}
	if !strings.Contains(message, "refresh-interval must be >= 1, got 0") {
		t.Fatalf("expected refresh-interval validation error, got %q", message)
	}
	if !strings.Contains(message, "job-history-limit must be >= 1, got 0") {
		t.Fatalf("expected job-history-limit validation error, got %q", message)
	}
}

func TestConfig_String_DoesNotExposePassword(t *testing.T) {
	cfg := &Config{
		ListenAddr:      ":3000",
		Namespace:       "prod",
		RefreshInterval: 5,
		JobHistoryLimit: 7,
		AuthUsername:    "user",
		AuthPassword:    "super-secret",
	}

	message := cfg.String()
	if !strings.Contains(message, `Config{ListenAddr:":3000" Namespace:"prod" RefreshInterval:5 JobHistoryLimit:7 AuthUsername:"user"}`) {
		t.Fatalf("unexpected string form: %q", message)
	}
	if strings.Contains(message, "super-secret") {
		t.Fatalf("config string should not expose password: %q", message)
	}
	if strings.Contains(message, "AuthPassword") {
		t.Fatalf("config string should not mention AuthPassword: %q", message)
	}
}
