package monitor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/alert"
	"github.com/serverwatch/serverwatch/server/internal/repository"
	"github.com/serverwatch/serverwatch/server/internal/service"
)

type Scheduler struct {
	Repository   *repository.MonitorRepository
	Checker      *HTTPChecker
	TCPChecker   *TCPChecker
	SSLChecker   *SSLChecker
	PingChecker  *PingChecker
	DNSChecker   *DNSChecker
	Interval     time.Duration
	AlertService *alert.Service
}

func NewScheduler(
	repository *repository.MonitorRepository,
) *Scheduler {
	return &Scheduler{
		Repository:  repository,
		Checker:     NewHTTPChecker(),
		TCPChecker:  NewTCPChecker(),
		SSLChecker:  NewSSLChecker(),
		PingChecker: NewPingChecker(),
		DNSChecker:  NewDNSChecker(),
		Interval:    5 * time.Second,
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
			var sslResult *SSLCheckResult

			switch monitorItem.MonitorType {
			case "http", "https":
				result = s.Checker.Check(ctx, monitorItem)
			case "tcp":
				result = s.TCPChecker.Check(ctx, monitorItem)
			case "ssl":
				checked := s.SSLChecker.CheckCertificate(ctx, monitorItem)
				sslResult = &checked
				result = checked.Result
			case "ping":
				result = s.PingChecker.Check(ctx, monitorItem)
			case "dns":
				result = s.DNSChecker.Check(ctx, monitorItem).Result
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
			if s.AlertService != nil {
				observation := service.AlertObservation{
					Monitor: monitorItem, Status: result.Status,
					ResponseTimeMS: result.ResponseTimeMS, StatusCode: result.StatusCode,
					Message: result.Message, CheckedAt: time.Now(),
				}
				if sslResult != nil && sslResult.CertificateValid {
					observation.SSL = &service.SSLAlertMetadata{
						Subject: sslResult.Subject, Issuer: sslResult.Issuer,
						NotAfter: sslResult.NotAfter, DaysRemaining: sslResult.DaysRemaining,
					}
				}
				s.AlertService.Process(ctx, observation)
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
