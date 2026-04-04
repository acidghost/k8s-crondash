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

func TestDashboard_ContainsTableHeaders(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, ""))
	if !strings.Contains(html, "<th>Name</th>") {
		t.Error("should contain Name header")
	}
	if !strings.Contains(html, "<th>Schedule</th>") {
		t.Error("should contain Schedule header")
	}
	if !strings.Contains(html, "<th>Namespace</th>") {
		t.Error("should contain Namespace header when showNamespace=true")
	}
	if !strings.Contains(html, "<th>Status</th>") {
		t.Error("should contain Status header")
	}
	if !strings.Contains(html, "<th>Last Success</th>") {
		t.Error("should contain Last Success header")
	}
	if !strings.Contains(html, "<th>Last Failure</th>") {
		t.Error("should contain Last Failure header")
	}
	if !strings.Contains(html, "<th>Active Jobs</th>") {
		t.Error("should contain Active Jobs header")
	}
}

func TestDashboard_NamespaceColumnHidden(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, false, 5, ""))
	if strings.Contains(html, "<th>Namespace</th>") {
		t.Error("should NOT contain Namespace header when showNamespace=false")
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

func TestCronJobTableBody_RendersJobData(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
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
	html := renderToString(t, CronJobTableBody(jobs, true))

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
		t.Error("should show Suspended badge")
	}
	if !strings.Contains(html, "2025-01-15 10:30:00") {
		t.Error("should format last success time")
	}
}

func TestCronJobTableBody_RunningBadge(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "running-job", Running: true, ActiveJobs: 2},
	}
	html := renderToString(t, CronJobTableBody(jobs, false))

	if !strings.Contains(html, "Running") {
		t.Error("should show Running badge")
	}
}

func TestCronJobTableBody_IdleBadge(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "idle-job"},
	}
	html := renderToString(t, CronJobTableBody(jobs, false))

	if !strings.Contains(html, "Idle") {
		t.Error("should show Idle badge for non-suspended, non-running jobs")
	}
}

func TestCronJobTableBody_NamespaceHidden(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "job-a", Namespace: "secret-ns"},
	}
	html := renderToString(t, CronJobTableBody(jobs, false))

	if strings.Contains(html, "secret-ns") {
		t.Error("should NOT contain namespace when showNamespace=false")
	}
}

func TestCronJobTableBody_NilTimes(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "fresh-job", LastSuccess: nil, LastFailure: nil},
	}
	html := renderToString(t, CronJobTableBody(jobs, false))

	if !strings.Contains(html, "—") {
		t.Error("nil times should render as dash")
	}
}

func TestCronJobTableBody_Empty(t *testing.T) {
	html := renderToString(t, CronJobTableBody([]k8s.CronJobDisplay{}, false))
	if !strings.Contains(html, "No CronJobs found") {
		t.Error("empty table body should show no data message")
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
