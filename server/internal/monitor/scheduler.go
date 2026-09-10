package monitor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/repository"
	"github.com/serverwatch/serverwatch/server/internal/service"
)

type Scheduler struct {
	Repository *repository.MonitorRepository
	Checker    *HTTPChecker
	TCPChecker *TCPChecker
	SSLChecker *SSLChecker
	Interval   time.Duration
}

func NewScheduler(
	repository *repository.MonitorRepository,
) *Scheduler {
	return &Scheduler{
		Repository: repository,
		Checker:    NewHTTPChecker(),
		TCPChecker: NewTCPChecker(),
		SSLChecker: NewSSLChecker(),
		Interval:   5 * time.Second,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	log.Println("monitor scheduler started")

	s.runOnce(ctx)

	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("monitor scheduler stopped")
			return

		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	monitors, err := s.Repository.ListDue(ctx, 50)

	if err != nil {
		log.Printf(
			"monitor scheduler: failed to load monitors: %v",
			err,
		)
		return
	}

	if len(monitors) == 0 {
		return
	}

	const maxConcurrentChecks = 5

	semaphore := make(
		chan struct{},
		maxConcurrentChecks,
	)

	var wg sync.WaitGroup

	for _, item := range monitors {
		monitorItem := item

		wg.Add(1)

		go func() {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() {
					<-semaphore
				}()

			case <-ctx.Done():
				return
			}

			var result service.CreateMonitorResultInput

			switch monitorItem.MonitorType {
			case "http", "https":
				result = s.Checker.Check(ctx, monitorItem)
			case "tcp":
				result = s.TCPChecker.Check(ctx, monitorItem)
			case "ssl":
				result = s.SSLChecker.Check(ctx, monitorItem)
			default:
				log.Printf(
					"monitor %d (%s): unsupported monitor type %q",
					monitorItem.ID,
					monitorItem.Name,
					monitorItem.MonitorType,
				)
				return
			}

			if err := s.Repository.SaveResult(
				ctx,
				result,
			); err != nil {
				log.Printf(
					"monitor %d (%s): failed to save result: %v",
					monitorItem.ID,
					monitorItem.Name,
					err,
				)
				return
			}

			log.Printf(
				"monitor %d (%s): %s %dms",
				monitorItem.ID,
				monitorItem.Name,
				result.Status,
				result.ResponseTimeMS,
			)
		}()
	}

	wg.Wait()
}
