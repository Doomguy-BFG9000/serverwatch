package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

type HTTPChecker struct {
	Client *http.Client
}

func NewHTTPChecker() *HTTPChecker {
	return &HTTPChecker{
		Client: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
	}
}

func (c *HTTPChecker) Check(
	ctx context.Context,
	monitor service.Monitor,
) service.CreateMonitorResultInput {
	timeout := time.Duration(monitor.TimeoutSeconds) * time.Second

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()

	req, err := http.NewRequestWithContext(
		checkCtx,
		http.MethodGet,
		monitor.Target,
		nil,
	)

	if err != nil {
		return service.CreateMonitorResultInput{
			MonitorID: monitor.ID,
			Status:    "down",
			Message:   fmt.Sprintf("request creation failed: %v", err),
		}
	}

	req.Header.Set(
		"User-Agent",
		"ServerWatch-Community/0.1",
	)

	resp, err := c.Client.Do(req)

	elapsed := time.Since(started).Milliseconds()

	if err != nil {
		return service.CreateMonitorResultInput{
			MonitorID:      monitor.ID,
			Status:         "down",
			ResponseTimeMS: elapsed,
			Message:        err.Error(),
		}
	}

	defer resp.Body.Close()

	statusCode := resp.StatusCode

	status := "up"

	if statusCode < 200 || statusCode >= 400 {
		status = "down"
	}

	return service.CreateMonitorResultInput{
		MonitorID:      monitor.ID,
		Status:         status,
		ResponseTimeMS: elapsed,
		StatusCode:     &statusCode,
		Message:        resp.Status,
	}
}
