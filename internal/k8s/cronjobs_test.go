package k8s

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListCronJobs_Empty(t *testing.T) {
	clientset := fake.NewClientset()

	cronJobs, err := ListCronJobs(context.Background(), clientset, "", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cronJobs) != 0 {
		t.Fatalf("expected 0 cronjobs, got %d", len(cronJobs))
	}
}

func TestListCronJobs_WithCronJobs(t *testing.T) {
	suspend := false
	clientset := fake.NewClientset()

	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cj",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			Suspend:  &suspend,
		},
	}
	_, err := clientset.BatchV1().CronJobs("default").Create(context.Background(), cj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test cronjob: %v", err)
	}

	cronJobs, err := ListCronJobs(context.Background(), clientset, "default", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cronJobs) != 1 {
		t.Fatalf("expected 1 cronjob, got %d", len(cronJobs))
	}
	if cronJobs[0].Name != "test-cj" {
		t.Errorf("expected name test-cj, got %s", cronJobs[0].Name)
	}
	if cronJobs[0].Schedule != "*/5 * * * *" {
		t.Errorf("expected schedule */5 * * * *, got %s", cronJobs[0].Schedule)
	}
	if cronJobs[0].Suspended {
		t.Error("expected cronjob to not be suspended")
	}
}

func TestListCronJobs_Suspended(t *testing.T) {
	suspend := true
	clientset := fake.NewClientset()

	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "suspended-cj",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			Suspend:  &suspend,
		},
	}
	_, err := clientset.BatchV1().CronJobs("default").Create(context.Background(), cj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test cronjob: %v", err)
	}

	cronJobs, err := ListCronJobs(context.Background(), clientset, "default", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cronJobs[0].Suspended {
		t.Error("expected cronjob to be suspended")
	}
}

func TestListCronJobs_WithJobHistory(t *testing.T) {
	suspend := false
	clientset := fake.NewClientset()

	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cj",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			Suspend:  &suspend,
		},
	}
	_, err := clientset.BatchV1().CronJobs("default").Create(context.Background(), cj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test cronjob: %v", err)
	}

	successTime := metav1.NewTime(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))
	failTime := metav1.NewTime(time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC))

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cj-28654670",
			Namespace: "default",
			Labels: map[string]string{
				"batch.kubernetes.io/cronjob": "test-cj",
			},
		},
		Spec: batchv1.JobSpec{},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobComplete,
					Status:             "True",
					LastTransitionTime: successTime,
				},
			},
		},
	}
	_, err = clientset.BatchV1().Jobs("default").Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test job: %v", err)
	}

	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cj-28654669",
			Namespace: "default",
			Labels: map[string]string{
				"batch.kubernetes.io/cronjob": "test-cj",
			},
		},
		Spec: batchv1.JobSpec{},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobFailed,
					Status:             "True",
					LastTransitionTime: failTime,
				},
			},
		},
	}
	_, err = clientset.BatchV1().Jobs("default").Create(context.Background(), failedJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create failed test job: %v", err)
	}

	cronJobs, err := ListCronJobs(context.Background(), clientset, "default", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cronJobs) != 1 {
		t.Fatalf("expected 1 cronjob, got %d", len(cronJobs))
	}

	display := cronJobs[0]
	if display.LastSuccess == nil {
		t.Fatal("expected last success time to be set")
	}
	if !display.LastSuccess.Equal(successTime.Time) {
		t.Errorf("expected last success at %v, got %v", successTime.Time, display.LastSuccess)
	}
	if display.LastFailure == nil {
		t.Fatal("expected last failure time to be set")
	}
	if !display.LastFailure.Equal(failTime.Time) {
		t.Errorf("expected last failure at %v, got %v", failTime.Time, display.LastFailure)
	}
}
