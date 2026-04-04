package views

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/acidghost/k8s-crondash/internal/k8s"
)

func renderToString(t *testing.T, comp templ.Component) string {
	t.Helper()
	buf := new(strings.Builder)
	err := comp.Render(context.Background(), buf)
	if err != nil {
		t.Fatalf("failed to render: %v", err)
	}
	return buf.String()
}

func TestDashboard_ContainsCardGrid(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, ""))
	if !strings.Contains(html, `class="grid spacious"`) {
		t.Error("should contain grid class")
	}
	if !strings.Contains(html, `data-cols@s="1"`) {
		t.Error("should contain responsive small cols")
	}
	if !strings.Contains(html, `data-cols@l="3"`) {
		t.Error("should contain responsive large cols")
	}
}

func TestDashboard_ContainsHTMXAttributes(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 10, ""))
	if !strings.Contains(html, `hx-get="/cronjobs"`) {
		t.Error("should contain hx-get attribute")
	}
	if !strings.Contains(html, `every 10s`) {
		t.Error("should contain refresh interval in hx-trigger")
	}
	if !strings.Contains(html, `hx-swap="innerHTML"`) {
		t.Error("should contain hx-swap attribute")
	}
}

func TestDashboard_EmptyData_ShowsEmptyState(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, "default"))
	if !strings.Contains(html, "No CronJobs found") {
		t.Error("should show empty state for no jobs")
	}
	if !strings.Contains(html, "default") {
		t.Error("should show namespace name in empty state")
	}
}

func TestDashboard_EmptyData_AllNamespaces(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, ""))
	if !strings.Contains(html, "all") {
		t.Error("should show 'all' when namespace is empty")
	}
}

func TestCronJobCards_RendersJobData(t *testing.T) {
	now := time.Now().Add(-5 * time.Minute)
	jobs := []k8s.CronJobDisplay{
		{
			Name:        "backup-job",
			Namespace:   "prod",
			Schedule:    "0 2 * * *",
			Suspended:   true,
			ActiveJobs:  0,
			LastSuccess: &now,
		},
	}
	html := renderToString(t, CronJobCards(jobs, true))

	if !strings.Contains(html, "backup-job") {
		t.Error("should contain job name")
	}
	if !strings.Contains(html, "prod") {
		t.Error("should contain namespace when showNamespace=true")
	}
	if !strings.Contains(html, "0 2 * * *") {
		t.Error("should contain schedule")
	}
	if !strings.Contains(html, "Suspended") {
		t.Error("should show Suspended chip")
	}
	if !strings.Contains(html, "chip warn") {
		t.Error("suspended job should have chip warn class")
	}
}

func TestCronJobCards_RunningChip(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "running-job", ActiveJobs: 2},
	}
	html := renderToString(t, CronJobCards(jobs, false))

	if !strings.Contains(html, "Running") {
		t.Error("should show Running chip")
	}
	if !strings.Contains(html, "chip ok") {
		t.Error("running job should have chip ok class")
	}
	if !strings.Contains(html, "status-dot--running") {
		t.Error("running job should have pulsing dot")
	}
	if !strings.Contains(html, "2 active") {
		t.Error("should show active job count")
	}
}

func TestCronJobCards_IdleChip(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "idle-job"},
	}
	html := renderToString(t, CronJobCards(jobs, false))

	if !strings.Contains(html, "Idle") {
		t.Error("should show Idle chip for non-suspended, non-running jobs")
	}
	if !strings.Contains(html, "chip plain") {
		t.Error("idle job should have chip plain class")
	}
}

func TestCronJobCards_NamespaceHidden(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "job-a", Namespace: "secret-ns"},
	}
	html := renderToString(t, CronJobCards(jobs, false))

	if strings.Contains(html, "secret-ns") {
		t.Error("should NOT contain namespace when showNamespace=false")
	}
}

func TestCronJobCards_NilTimes(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "fresh-job", LastSuccess: nil, LastFailure: nil},
	}
	html := renderToString(t, CronJobCards(jobs, false))

	if !strings.Contains(html, "—") {
		t.Error("nil times should render as dash")
	}
}

func TestCronJobCards_Empty(t *testing.T) {
	html := renderToString(t, CronJobCards([]k8s.CronJobDisplay{}, false))
	if !strings.Contains(html, "No CronJobs found") {
		t.Error("empty cards should show no data message")
	}
}

func TestCronJobCards_FormattedTime(t *testing.T) {
	ts := time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC)
	jobs := []k8s.CronJobDisplay{
		{Name: "test-job", LastSuccess: &ts},
	}
	html := renderToString(t, CronJobCards(jobs, false))

	if !strings.Contains(html, "2025-03-15 10:30:00") {
		t.Error("should format last success time")
	}
}

func TestEmptyState_Message(t *testing.T) {
	html := renderToString(t, EmptyState("production"))
	if !strings.Contains(html, "production") {
		t.Error("should contain namespace name")
	}
	if !strings.Contains(html, "No CronJobs found") {
		t.Error("should contain no cronjobs message")
	}
}

func TestEmptyState_AllNamespaces(t *testing.T) {
	html := renderToString(t, EmptyState(""))
	if !strings.Contains(html, "all") {
		t.Error("should show 'all' when namespace is empty")
	}
}
