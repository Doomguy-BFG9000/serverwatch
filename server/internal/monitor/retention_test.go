package monitor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fakePruner struct {
	calls    int32
	lastDays int
	deleted  int64
	err      error
}

func (f *fakePruner) PruneResults(ctx context.Context, retentionDays int) (int64, error) {
	atomic.AddInt32(&f.calls, 1)
	f.lastDays = retentionDays
	return f.deleted, f.err
}

func TestRetentionJobDisabledWhenRetentionNonPositive(t *testing.T) {
	store := &fakePruner{}
	job := &RetentionJob{Store: store, RetentionDays: 0, Interval: time.Millisecond}

	done := make(chan struct{})
	go func() {
		job.Run(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disabled job must return immediately")
	}

	if calls := atomic.LoadInt32(&store.calls); calls != 0 {
		t.Fatalf("disabled job must not call PruneResults, called %d times", calls)
	}
}

func TestRetentionJobRunsImmediately(t *testing.T) {
	store := &fakePruner{deleted: 3}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	job := &RetentionJob{Store: store, RetentionDays: 14, Interval: time.Hour}
	go job.Run(ctx)

	deadline := time.After(time.Second)
	for atomic.LoadInt32(&store.calls) == 0 {
		select {
		case <-deadline:
			t.Fatal("job did not run at startup")
		case <-time.After(time.Millisecond):
		}
	}

	if store.lastDays != 14 {
		t.Fatalf("retention days passed to store = %d, want 14", store.lastDays)
	}
}

func TestRetentionJobNilStoreIsNoop(t *testing.T) {
	job := &RetentionJob{Store: nil, RetentionDays: 7, Interval: time.Millisecond}
	done := make(chan struct{})
	go func() {
		job.Run(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("nil store must return immediately")
	}
}

func TestNewRetentionJobDefaults(t *testing.T) {
	job := NewRetentionJob(&fakePruner{}, 30)
	if job.Interval != 24*time.Hour {
		t.Fatalf("default interval = %v, want 24h", job.Interval)
	}
	if job.RetentionDays != 30 {
		t.Fatalf("retention days = %d, want 30", job.RetentionDays)
	}
}
