package state

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/acidghost/k8s-crondash/internal/k8s"
	"k8s.io/client-go/kubernetes"
)

type Store struct {
	clientset       kubernetes.Interface
	namespace       string
	jobHistoryLimit int
	refreshInterval time.Duration

	mu       sync.RWMutex
	cache    []k8s.CronJobDisplay
	lastSync time.Time
	ready    atomic.Bool
}

func NewStore(ctx context.Context, clientset kubernetes.Interface, namespace string, refreshInterval time.Duration, jobHistoryLimit int) *Store {
	s := &Store{
		clientset:       clientset,
		namespace:       namespace,
		jobHistoryLimit: jobHistoryLimit,
		refreshInterval: refreshInterval,
	}

	go s.run(ctx)
	return s
}

func (s *Store) ListCronJobs(_ context.Context) ([]k8s.CronJobDisplay, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]k8s.CronJobDisplay, len(s.cache))
	copy(out, s.cache)
	return out, nil
}

func (s *Store) IsReady() bool {
	return s.ready.Load()
}

func (s *Store) LastSync() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSync
}

func (s *Store) run(ctx context.Context) {
	s.sync(ctx)

	ticker := time.NewTicker(s.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("state store polling stopped")
			return
		case <-ticker.C:
			s.sync(ctx)
		}
	}
}

func (s *Store) sync(ctx context.Context) {
	cronJobs, err := k8s.ListCronJobs(ctx, s.clientset, s.namespace, s.jobHistoryLimit)
	if err != nil {
		slog.Error("failed to sync cronjobs", "error", err)
		return
	}

	s.mu.Lock()
	s.cache = cronJobs
	s.lastSync = time.Now()
	s.mu.Unlock()

	s.ready.Store(true)
	slog.Info("synced cronjobs", "count", len(cronJobs))
}
