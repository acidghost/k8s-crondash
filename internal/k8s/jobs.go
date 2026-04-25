package k8s

import (
	"context"
	"fmt"
	"sort"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ListJobs(ctx context.Context, clientset kubernetes.Interface, namespace, cronJobName string, limit int) ([]batchv1.Job, error) {
	return listChildJobs(ctx, clientset, namespace, cronJobName, limit)
}

func listChildJobs(ctx context.Context, clientset kubernetes.Interface, namespace, cronJobName string, limit int) ([]batchv1.Job, error) {
	jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list jobs for cronjob %s/%s: %w", namespace, cronJobName, err)
	}

	filtered := make([]batchv1.Job, 0, len(jobs.Items))
	for _, job := range jobs.Items {
		if isChildJob(job, cronJobName) {
			filtered = append(filtered, job)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		left := filtered[i].CreationTimestamp.Time
		right := filtered[j].CreationTimestamp.Time
		if left.Equal(right) {
			return filtered[i].Name > filtered[j].Name
		}
		return left.After(right)
	})

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

func isChildJob(job batchv1.Job, cronJobName string) bool {
	for _, owner := range job.OwnerReferences {
		if owner.Kind == "CronJob" && owner.Name == cronJobName {
			return true
		}
	}

	if job.Labels == nil {
		return false
	}

	if job.Labels["batch.kubernetes.io/cronjob"] == cronJobName {
		return true
	}
	if job.Labels["batch.kubernetes.io/cronjob-name"] == cronJobName {
		return true
	}
	if job.Labels["cronjob.kubernetes.io/instance"] == cronJobName {
		return true
	}

	return false
}

func isJobRunning(job *batchv1.Job) bool {
	if job.Status.Active > 0 {
		return true
	}
	if job.Status.StartTime != nil && job.Status.CompletionTime == nil {
		return true
	}
	return false
}
