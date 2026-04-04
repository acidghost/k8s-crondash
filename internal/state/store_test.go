package state

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestStore_IsReady_AfterSync(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore(ctx, clientset, "default", 50*time.Millisecond, 5)

	for i := 0; i < 50; i++ {
		if store.IsReady() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("store never became ready")
}

func TestStore_ListCronJobs_ReturnsData(t *testing.T) {
	clientset := fake.NewClientset()

	suspend := false
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
		t.Fatalf("failed to create cronjob: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore(ctx, clientset, "default", 50*time.Millisecond, 5)

	for i := 0; i < 50; i++ {
		jobs := store.ListCronJobs()
		if len(jobs) > 0 {
			if jobs[0].Name != "test-cj" {
				t.Errorf("expected name test-cj, got %s", jobs[0].Name)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("store never synced cronjobs")
}

func TestStore_ListCronJobs_ReturnsCopy(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore(ctx, clientset, "default", 50*time.Millisecond, 5)

	for i := 0; i < 50; i++ {
		if store.IsReady() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	a := store.ListCronJobs()
	b := store.ListCronJobs()

	if len(a) != len(b) {
		t.Fatalf("expected same length, got %d and %d", len(a), len(b))
	}

	if len(a) > 0 {
		a[0].Name = "mutated"
		if b[0].Name == "mutated" {
			t.Error("ListCronJobs should return a copy, not a reference to internal state")
		}
	}
}

func TestStore_LastSync_Updates(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore(ctx, clientset, "default", 50*time.Millisecond, 5)

	for i := 0; i < 50; i++ {
		if store.IsReady() {
			lastSync := store.LastSync()
			if lastSync.IsZero() {
				t.Error("expected lastSync to be set after ready")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("store never became ready")
}

func TestStore_StopsOnContextCancel(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithCancel(context.Background())

	store := NewStore(ctx, clientset, "default", 50*time.Millisecond, 5)

	for i := 0; i < 50; i++ {
		if store.IsReady() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	time.Sleep(200 * time.Millisecond)

	lastSync := store.LastSync()
	time.Sleep(100 * time.Millisecond)
	lastSync2 := store.LastSync()

	if !lastSync2.Equal(lastSync) {
		t.Error("store should have stopped syncing after context cancel")
	}
}
