package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/alert"
	"github.com/serverwatch/serverwatch/server/internal/config"
	httpapi "github.com/serverwatch/serverwatch/server/internal/http"
	monitorengine "github.com/serverwatch/serverwatch/server/internal/monitor"
	"github.com/serverwatch/serverwatch/server/internal/repository"
	"github.com/serverwatch/serverwatch/server/internal/storage"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()
	db, err := storage.NewPostgresPool(ctx)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	healthHandler := &httpapi.HealthHandler{
		DB: db,
	}
	monitorRepository := &repository.MonitorRepository{
		DB: db,
	}
	groupRepository := &repository.GroupRepository{
		DB: db,
	}
	authRepository := &repository.AuthRepository{
		DB: db,
	}
	monitorScheduler := monitorengine.NewScheduler(
		monitorRepository,
	)
	monitorScheduler.AlertService = &alert.Service{
		Store:   monitorRepository,
		Sender:  alert.SMTPSender{Config: cfg.AlertEmail},
		Enabled: cfg.AlertEmail.Enabled,
	}

	schedulerCtx, schedulerCancel := context.WithCancel(
		context.Background(),
	)
	defer schedulerCancel()

	go monitorScheduler.Run(schedulerCtx)

	monitorHandler := &httpapi.MonitorHandler{
		Repository:  monitorRepository,
		SSLChecker:  monitorengine.NewSSLChecker(),
		DNSChecker:  monitorengine.NewDNSChecker(),
		PortChecker: monitorengine.NewPortChecker(),
	}
	groupHandler := &httpapi.GroupHandler{
		Repository: groupRepository,
	}
	alertHandler := &httpapi.AlertHandler{Config: cfg.AlertEmail}
	authHandler := &httpapi.AuthHandler{
		Store:        authRepository,
		AuthEnabled:  cfg.AuthEnabled,
		SessionTTL:   cfg.SessionTTL,
		CookieSecure: cfg.CookieSecure,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("GET /ready", healthHandler.Ready)
	mux.HandleFunc("GET /api/v1/auth/status", authHandler.Status)
	mux.HandleFunc("POST /api/v1/auth/setup", authHandler.Setup)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/v1/auth/me", authHandler.Me)
	mux.HandleFunc("GET /api/v1/alerts/status", alertHandler.Status)
	mux.HandleFunc("GET /api/v1/groups", groupHandler.List)
	mux.HandleFunc("POST /api/v1/groups", groupHandler.Create)
	mux.HandleFunc("PUT /api/v1/groups/{id}", groupHandler.Update)
	mux.HandleFunc("DELETE /api/v1/groups/{id}", groupHandler.Delete)
	mux.HandleFunc("POST /api/v1/monitors", monitorHandler.Create)
	mux.HandleFunc("GET /api/v1/monitors", monitorHandler.List)
	mux.HandleFunc("GET /api/v1/monitors/{id}", monitorHandler.Get)
	mux.HandleFunc("DELETE /api/v1/monitors/{id}", monitorHandler.Delete)
	mux.HandleFunc("GET /api/v1/monitors/status", monitorHandler.Status)
	mux.HandleFunc("GET /api/v1/monitors/{id}/results", monitorHandler.Results)
	mux.HandleFunc("GET /api/v1/monitors/{id}/ssl", monitorHandler.SSL)
	mux.HandleFunc("GET /api/v1/monitors/{id}/dns", monitorHandler.DNS)
	mux.HandleFunc("GET /api/v1/monitors/{id}/ports", monitorHandler.Ports)
	mux.HandleFunc("PUT /api/v1/monitors/{id}", monitorHandler.Update)
	mux.HandleFunc(
		"PUT /api/v1/monitors/{id}/maintenance",
		monitorHandler.SetMaintenance,
	)
	mux.HandleFunc(
		"PUT /api/v1/monitors/{id}/favorite",
		monitorHandler.SetFavorite,
	)
	mux.HandleFunc(
		"GET /api/v1/monitors/{id}/stats",
		monitorHandler.Stats,
	)
	mux.HandleFunc(
		"GET /api/v1/monitors/{id}/chart",
		monitorHandler.Chart,
	)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewAuthMiddleware(cfg.AuthEnabled, cfg.APIToken, authRepository, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("ServerWatch API listening on %s", server.Addr)

		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(
		stop,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	<-stop

	log.Println("ServerWatch API shutting down...")

	schedulerCancel()
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	log.Println("ServerWatch API stopped")
}
