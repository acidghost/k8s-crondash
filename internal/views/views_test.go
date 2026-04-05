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
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, "", "", ""))
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
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 10, "", "", ""))
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

func TestDashboard_ContainsRefreshButton(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, "", "", ""))
	if !strings.Contains(html, `href="/"`) {
		t.Error("should contain refresh link")
	}
	if !strings.Contains(html, "Refresh") {
		t.Error("should contain Refresh text")
	}
}

func TestDashboard_FlashBanner(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, "", "Job triggered", "ok"))
	if !strings.Contains(html, "Job triggered") {
		t.Error("should contain flash message")
	}
	if !strings.Contains(html, "chip ok") {
		t.Error("success flash should have chip ok class")
	}
}

func TestDashboard_FlashBannerError(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, "", "Something failed", "bad"))
	if !strings.Contains(html, "Something failed") {
		t.Error("should contain flash message")
	}
	if !strings.Contains(html, "chip bad") {
		t.Error("error flash should have chip bad class")
	}
}

func TestDashboard_NoFlashBanner(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, "", "", ""))
	if strings.Contains(html, "chip ok") || strings.Contains(html, "chip bad") {
		t.Error("should not render flash banner when flash is empty")
	}
}

func TestDashboard_EmptyData_ShowsEmptyState(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, "default", "", ""))
	if !strings.Contains(html, "No CronJobs found") {
		t.Error("should show empty state for no jobs")
	}
	if !strings.Contains(html, "default") {
		t.Error("should show namespace name in empty state")
	}
}

func TestDashboard_EmptyData_AllNamespaces(t *testing.T) {
	html := renderToString(t, Dashboard([]k8s.CronJobDisplay{}, true, 5, "", "", ""))
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

func TestCronJobCards_TriggerButtonIsLink(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "idle-job", Namespace: "default"},
	}
	html := renderToString(t, CronJobCards(jobs, false))

	if !strings.Contains(html, `href="/trigger-confirm/default/idle-job"`) {
		t.Error("trigger button should be an <a> tag with href for no-JS fallback")
	}
	if !strings.Contains(html, `hx-get="/trigger-confirm/default/idle-job"`) {
		t.Error("trigger link should have hx-get for HTMX")
	}
	if !strings.Contains(html, `class="<button>"`) {
		t.Error("trigger link should have class=<button> for styling")
	}
}

func TestCronJobCards_TriggerButtonSuspendedIsDisabled(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "suspended-job", Namespace: "default", Suspended: true},
	}
	html := renderToString(t, CronJobCards(jobs, false))

	if !strings.Contains(html, "<button disabled") {
		t.Error("suspended job should have disabled button")
	}
}

func TestCronJobCards_TriggerButtonRunningIsLink(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "running-job", Namespace: "ns1", ActiveJobs: 1},
	}
	html := renderToString(t, CronJobCards(jobs, false))

	if !strings.Contains(html, `href="/trigger-confirm/ns1/running-job"`) {
		t.Error("running job trigger should be an <a> tag with href")
	}
	if !strings.Contains(html, "⚠ Trigger") {
		t.Error("running job trigger should show warning icon")
	}
}

func TestCronJobCards_NamespaceHidden(t *testing.T) {
	jobs := []k8s.CronJobDisplay{
		{Name: "job-a", Namespace: "secret-ns"},
	}
	html := renderToString(t, CronJobCards(jobs, false))

	if strings.Contains(html, "secondary-font\"><span class=\"mono-font\">secret-ns") {
		t.Error("should NOT render namespace display line when showNamespace=false")
	}
	if !strings.Contains(html, "/trigger-confirm/secret-ns/job-a") {
		t.Error("trigger button URL should still contain namespace")
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

func TestTriggerConfirmModal_ContainsDialog(t *testing.T) {
	job := k8s.CronJobDisplay{
		Name:      "backup-job",
		Namespace: "prod",
		Schedule:  "0 2 * * *",
	}
	html := renderToString(t, TriggerConfirmModal(job))

	if !strings.Contains(html, "<dialog") {
		t.Error("should contain dialog element")
	}
	if !strings.Contains(html, "backup-job") {
		t.Error("should contain job name")
	}
	if !strings.Contains(html, "prod") {
		t.Error("should contain namespace")
	}
	if !strings.Contains(html, "Trigger CronJob?") {
		t.Error("should contain confirm heading")
	}
	if !strings.Contains(html, "/trigger/prod/backup-job") {
		t.Error("should contain trigger POST URL")
	}
	if !strings.Contains(html, "Confirm") {
		t.Error("should contain confirm button")
	}
	if !strings.Contains(html, "Cancel") {
		t.Error("should contain cancel link")
	}
}

func TestTriggerConfirmModal_HasFormAndLinkCancel(t *testing.T) {
	job := k8s.CronJobDisplay{
		Name:      "test-job",
		Namespace: "default",
	}
	html := renderToString(t, TriggerConfirmModal(job))

	if !strings.Contains(html, `<form method="POST"`) {
		t.Error("confirm should be wrapped in a form for no-JS fallback")
	}
	if !strings.Contains(html, `action="/trigger/default/test-job"`) {
		t.Error("form should have action URL for no-JS fallback")
	}
	if !strings.Contains(html, `hx-post="/trigger/default/test-job"`) {
		t.Error("form should have hx-post for HTMX")
	}
	if !strings.Contains(html, `<a href="/"`) {
		t.Error("cancel should be a link with href for no-JS fallback")
	}
	if !strings.Contains(html, `onclick="event.preventDefault()`) {
		t.Error("cancel link should prevent default for JS modal close")
	}
}

func TestTriggerConfirmModal_RunningJob_ShowsWarning(t *testing.T) {
	job := k8s.CronJobDisplay{
		Name:       "running-job",
		Namespace:  "default",
		ActiveJobs: 1,
	}
	html := renderToString(t, TriggerConfirmModal(job))

	if !strings.Contains(html, "already running") {
		t.Error("should show running warning")
	}
	if !strings.Contains(html, "chip warn") {
		t.Error("warning should have chip warn class")
	}
}

func TestTriggerConfirmModal_IdleJob_NoWarning(t *testing.T) {
	job := k8s.CronJobDisplay{
		Name:      "idle-job",
		Namespace: "default",
	}
	html := renderToString(t, TriggerConfirmModal(job))

	if strings.Contains(html, "already running") {
		t.Error("should NOT show running warning for idle job")
	}
}

func TestTriggerConfirmPage_FullPage(t *testing.T) {
	job := k8s.CronJobDisplay{
		Name:      "backup-job",
		Namespace: "prod",
	}
	html := renderToString(t, TriggerConfirmPage(job))

	if !strings.Contains(html, "<!doctype html>") {
		t.Error("should render full page with layout")
	}
	if !strings.Contains(html, "Trigger CronJob?") {
		t.Error("should contain confirm heading")
	}
	if !strings.Contains(html, "backup-job") {
		t.Error("should contain job name")
	}
	if !strings.Contains(html, `<form method="POST" action="/trigger/prod/backup-job"`) {
		t.Error("should have form with action for no-JS submit")
	}
	if !strings.Contains(html, `<button type="submit">Confirm</button>`) {
		t.Error("should have submit button")
	}
	if !strings.Contains(html, `<a href="/" class="<button>">Cancel</a>`) {
		t.Error("cancel should be styled link to home")
	}
}

func TestTriggerConfirmPage_NoDialog(t *testing.T) {
	job := k8s.CronJobDisplay{
		Name:      "test-job",
		Namespace: "default",
	}
	html := renderToString(t, TriggerConfirmPage(job))

	if strings.Contains(html, "<dialog") {
		t.Error("full page should NOT use dialog element")
	}
	if strings.Contains(html, "hx-post") {
		t.Error("full page should NOT use HTMX attributes")
	}
	if strings.Contains(html, "onclick") {
		t.Error("full page should NOT use onclick handlers")
	}
}

func TestTriggerConfirmPage_RunningJob_ShowsWarning(t *testing.T) {
	job := k8s.CronJobDisplay{
		Name:       "running-job",
		Namespace:  "default",
		ActiveJobs: 2,
	}
	html := renderToString(t, TriggerConfirmPage(job))

	if !strings.Contains(html, "already running") {
		t.Error("should show running warning")
	}
}

func TestFlashBanner_Success(t *testing.T) {
	html := renderToString(t, FlashBanner("Job triggered", "ok"))
	if !strings.Contains(html, "Job triggered") {
		t.Error("should contain message")
	}
	if !strings.Contains(html, "chip ok") {
		t.Error("success should have chip ok class")
	}
}

func TestFlashBanner_Error(t *testing.T) {
	html := renderToString(t, FlashBanner("Something failed", "bad"))
	if !strings.Contains(html, "Something failed") {
		t.Error("should contain message")
	}
	if !strings.Contains(html, "chip bad") {
		t.Error("error should have chip bad class")
	}
}

func TestFlashBanner_Empty(t *testing.T) {
	html := renderToString(t, FlashBanner("", ""))
	if strings.Contains(html, "chip") {
		t.Error("should not render anything when message is empty")
	}
}

func TestToast_Success(t *testing.T) {
	html := renderToString(t, Toast("Job triggered", true))

	if !strings.Contains(html, "Job triggered") {
		t.Error("should contain message")
	}
	if !strings.Contains(html, "chip ok") {
		t.Error("success toast should have chip ok class")
	}
	if !strings.Contains(html, "toast-container") {
		t.Error("should target toast container")
	}
	if !strings.Contains(html, "hx-swap-oob") {
		t.Error("should have OOB swap attribute")
	}
	if !strings.Contains(html, "setTimeout") {
		t.Error("should contain auto-dismiss script")
	}
	if !strings.Contains(html, `id="modal-container"`) {
		t.Error("should clear modal container")
	}
}

func TestToast_Error(t *testing.T) {
	html := renderToString(t, Toast("Trigger failed: error", false))

	if !strings.Contains(html, "Trigger failed: error") {
		t.Error("should contain error message")
	}
	if !strings.Contains(html, "chip bad") {
		t.Error("error toast should have chip bad class")
	}
	if !strings.Contains(html, "hx-swap-oob") {
		t.Error("should have OOB swap attribute")
	}
	if !strings.Contains(html, "setTimeout") {
		t.Error("should contain auto-dismiss script")
	}
}
