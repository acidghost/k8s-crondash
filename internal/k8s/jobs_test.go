package k8s

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListJobs_Empty(t *testing.T) {
	clientset := fake.NewClientset()

	jobs, err := ListJobs(context.Background(), clientset, "default", "test-cj", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestListJobs_WithChildJobs(t *testing.T) {
	clientset := fake.NewClientset()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cj-28654670",
			Namespace: "default",
			Labels: map[string]string{
				"batch.kubernetes.io/cronjob": "test-cj",
			},
		},
		Spec:   batchv1.JobSpec{},
		Status: batchv1.JobStatus{},
	}
	_, err := clientset.BatchV1().Jobs("default").Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test job: %v", err)
	}

	unrelatedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-job",
			Namespace: "default",
			Labels: map[string]string{
				"batch.kubernetes.io/cronjob": "other-cj",
			},
		},
		Spec:   batchv1.JobSpec{},
		Status: batchv1.JobStatus{},
	}
	_, err = clientset.BatchV1().Jobs("default").Create(context.Background(), unrelatedJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create unrelated job: %v", err)
	}

	jobs, err := ListJobs(context.Background(), clientset, "default", "test-cj", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "test-cj-28654670" {
		t.Errorf("expected job name test-cj-28654670, got %s", jobs[0].Name)
	}
}

func TestIsJobRunning_Active(t *testing.T) {
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			Active: 1,
		},
	}
	if !isJobRunning(job) {
		t.Error("expected job with active=1 to be running")
	}
}

func TestIsJobRunning_StartTimeNoCompletion(t *testing.T) {
	now := metav1.Now()
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			StartTime: &now,
		},
	}
	if !isJobRunning(job) {
		t.Error("expected job with start time and no completion time to be running")
	}
}

func TestIsJobRunning_Completed(t *testing.T) {
	now := metav1.Now()
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			StartTime:      &now,
			CompletionTime: &now,
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: "True"},
			},
		},
	}
	if isJobRunning(job) {
		t.Error("expected completed job to not be running")
	}
}

func TestIsJobRunning_Idle(t *testing.T) {
	job := &batchv1.Job{
		Status: batchv1.JobStatus{},
	}
	if isJobRunning(job) {
		t.Error("expected idle job to not be running")
	}
}

func TestIsJobRunning_Failed(t *testing.T) {
	now := metav1.Now()
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			StartTime:      &now,
			CompletionTime: &now,
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: "True"},
			},
		},
	}
	if isJobRunning(job) {
		t.Error("expected failed job to not be running")
	}
}

func TestListJobs_RespectsLabelSelector(t *testing.T) {
	clientset := fake.NewClientset()

	for i := 0; i < 3; i++ {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cj-job-" + string(rune('0'+i)),
				Namespace: "default",
				Labels: map[string]string{
					"batch.kubernetes.io/cronjob": "test-cj",
				},
			},
			Spec:   batchv1.JobSpec{},
			Status: batchv1.JobStatus{},
		}
		_, err := clientset.BatchV1().Jobs("default").Create(context.Background(), job, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create test job %d: %v", i, err)
		}
	}

	otherJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-cj-job",
			Namespace: "default",
			Labels: map[string]string{
				"batch.kubernetes.io/cronjob": "other-cj",
			},
		},
		Spec:   batchv1.JobSpec{},
		Status: batchv1.JobStatus{},
	}
	_, err := clientset.BatchV1().Jobs("default").Create(context.Background(), otherJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create other job: %v", err)
	}

	jobs, err := ListJobs(context.Background(), clientset, "default", "test-cj", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}
}
