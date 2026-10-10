package monitor

import (
	"context"
	"log"
	"time"
)

// ResultPruner is the storage capability the retention job needs. Keeping it an
// interface avoids importing the repository package (which would create an
// import cycle) and makes the job trivially testable.
type ResultPruner interface {
	PruneResults(ctx context.Context, retentionDays int) (int64, error)
}

// RetentionJob bounds the growth of check history by deleting monitor_results
// rows older than RetentionDays, keeping the newest result per monitor. It runs
// once at startup and then every Interval (daily).
//
// A non-positive RetentionDays disables the job, and Run returns immediately.
// Only errors and non-zero deletion counts are logged, so an idle install does
// not produce daily noise.
type RetentionJob struct {
	Store         ResultPruner
	RetentionDays int
	Interval      time.Duration
}

// NewRetentionJob builds a daily retention job. It does not start it; call Run.
func NewRetentionJob(store ResultPruner, retentionDays int) *RetentionJob {
	return &RetentionJob{
		Store:         store,
		RetentionDays: retentionDays,
		Interval:      24 * time.Hour,
	}
}

// Run blocks, pruning once immediately and then on each tick, until the context
// is cancelled.
func (j *RetentionJob) Run(ctx context.Context) {
	if j.Store == nil || j.RetentionDays <= 0 {
		return
	}

	interval := j.Interval
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	log.Printf("result retention started: keeping %d days of check history", j.RetentionDays)
	j.runOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("result retention stopped")
			return
		case <-ticker.C:
			j.runOnce(ctx)
		}
	}
}

func (j *RetentionJob) runOnce(ctx context.Context) {
	deleted, err := j.Store.PruneResults(ctx, j.RetentionDays)
	if err != nil {
		log.Printf("result retention: prune failed: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("result retention: pruned %d old check results", deleted)
	}
}
