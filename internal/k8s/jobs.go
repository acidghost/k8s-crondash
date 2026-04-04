package k8s

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

func ListJobs(ctx context.Context, clientset kubernetes.Interface, namespace, cronJobName string, limit int) ([]batchv1.Job, error) {
	return listChildJobs(ctx, clientset, namespace, cronJobName, limit)
}

func listChildJobs(ctx context.Context, clientset kubernetes.Interface, namespace, cronJobName string, limit int) ([]batchv1.Job, error) {
	labelSelector := labels.Set{
		"batch.kubernetes.io/cronjob": cronJobName,
	}.AsSelector().String()

	jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list jobs for cronjob %s/%s: %w", namespace, cronJobName, err)
	}

	return jobs.Items, nil
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
