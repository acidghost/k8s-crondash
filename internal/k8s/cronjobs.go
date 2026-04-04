package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CronJobDisplay struct {
	Name        string
	Namespace   string
	Schedule    string
	Suspended   bool
	Running     bool
	LastSuccess *time.Time
	LastFailure *time.Time
	ActiveJobs  int
}

func ListCronJobs(ctx context.Context, clientset kubernetes.Interface, namespace string, jobHistoryLimit int) ([]CronJobDisplay, error) {
	cronJobs, err := clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list cronjobs: %w", err)
	}

	var displays []CronJobDisplay
	for _, cj := range cronJobs.Items {
		display := CronJobDisplay{
			Name:      cj.Name,
			Namespace: cj.Namespace,
			Schedule:  cj.Spec.Schedule,
			Suspended: cj.Spec.Suspend != nil && *cj.Spec.Suspend,
		}

		jobs, err := listChildJobs(ctx, clientset, cj.Namespace, cj.Name, jobHistoryLimit)
		if err != nil {
			slog.Warn("failed to list child jobs", "namespace", cj.Namespace, "cronjob", cj.Name, "error", err)
		} else {
			processJobs(jobs, &display)
		}

		for _, ref := range cj.Status.Active {
			_ = ref
			display.ActiveJobs++
		}

		displays = append(displays, display)
	}

	return displays, nil
}

func processJobs(jobs []batchv1.Job, display *CronJobDisplay) {
	for _, job := range jobs {
		if isJobRunning(&job) {
			display.Running = true
			display.ActiveJobs++
		}
		for _, c := range job.Status.Conditions {
			switch c.Type {
			case batchv1.JobComplete:
				if c.Status == "True" {
					t := c.LastTransitionTime.Time
					if display.LastSuccess == nil || t.After(*display.LastSuccess) {
						display.LastSuccess = &t
					}
				}
			case batchv1.JobFailed:
				if c.Status == "True" {
					t := c.LastTransitionTime.Time
					if display.LastFailure == nil || t.After(*display.LastFailure) {
						display.LastFailure = &t
					}
				}
			}
		}
	}
}
