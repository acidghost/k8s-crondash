package k8s

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

func TestTriggerCronJob_Success(t *testing.T) {
	suspend := false
	clientset := fake.NewClientset()

	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cj",
			Namespace: "default",
			UID:       "test-uid-123",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			Suspend:  &suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: setupPodTemplate(),
				},
			},
		},
	}
	_, err := clientset.BatchV1().CronJobs("default").Create(context.Background(), cj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create cronjob: %v", err)
	}

	err = TriggerCronJob(context.Background(), clientset, "default", "test-cj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jobs, err := clientset.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobs.Items) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs.Items))
	}

	job := jobs.Items[0]
	if job.Labels["batch.kubernetes.io/cronjob"] != "test-cj" {
		t.Errorf("expected cronjob label, got %v", job.Labels)
	}
	if len(job.OwnerReferences) != 1 {
		t.Fatalf("expected 1 owner reference, got %d", len(job.OwnerReferences))
	}
	ref := job.OwnerReferences[0]
	if ref.APIVersion != "batch/v1" {
		t.Errorf("expected apiVersion batch/v1, got %s", ref.APIVersion)
	}
	if ref.Kind != "CronJob" {
		t.Errorf("expected kind CronJob, got %s", ref.Kind)
	}
	if ref.Name != "test-cj" {
		t.Errorf("expected name test-cj, got %s", ref.Name)
	}
	if ref.UID != "test-uid-123" {
		t.Errorf("expected uid test-uid-123, got %s", ref.UID)
	}
}

func TestTriggerCronJob_Suspended(t *testing.T) {
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
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: setupPodTemplate(),
				},
			},
		},
	}
	_, err := clientset.BatchV1().CronJobs("default").Create(context.Background(), cj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create cronjob: %v", err)
	}

	err = TriggerCronJob(context.Background(), clientset, "default", "suspended-cj")
	if err == nil {
		t.Fatal("expected error for suspended cronjob")
	}
	if err.Error() != "cronjob default/suspended-cj is suspended" {
		t.Errorf("unexpected error message: %v", err)
	}

	jobs, _ := clientset.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	if len(jobs.Items) != 0 {
		t.Error("no job should be created for suspended cronjob")
	}
}

func TestTriggerCronJob_AlreadyRunning(t *testing.T) {
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
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: setupPodTemplate(),
				},
			},
		},
	}
	_, err := clientset.BatchV1().CronJobs("default").Create(context.Background(), cj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create cronjob: %v", err)
	}

	runningJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cj-28654670",
			Namespace: "default",
			Labels: map[string]string{
				"batch.kubernetes.io/cronjob": "test-cj",
			},
		},
		Spec: batchv1.JobSpec{},
		Status: batchv1.JobStatus{
			Active:    1,
			StartTime: &metav1.Time{Time: time.Now()},
		},
	}
	_, err = clientset.BatchV1().Jobs("default").Create(context.Background(), runningJob, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create running job: %v", err)
	}

	err = TriggerCronJob(context.Background(), clientset, "default", "test-cj")
	if err == nil {
		t.Fatal("expected error for already running cronjob")
	}
	if err.Error() != "cronjob default/test-cj already has a running job: test-cj-28654670" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTriggerCronJob_NotFound(t *testing.T) {
	clientset := fake.NewClientset()

	err := TriggerCronJob(context.Background(), clientset, "default", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent cronjob")
	}
}

func setupPodTemplate() corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "busybox",
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}
