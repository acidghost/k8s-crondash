package handlers

import (
	"context"

	"github.com/acidghost/k8s-crondash/internal/k8s"
)

type CronJobService interface {
	ListCronJobs(ctx context.Context) ([]k8s.CronJobDisplay, error)
}
