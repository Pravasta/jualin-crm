// Command api runs the Jualin CRM HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// No logger exists yet at this point — config failure is reported
		// to stderr directly and the process exits immediately (TD §3).
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	log := logger.New(cfg)

	router := newRouter(log)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
	}

	go func() {
		log.Info("server starting", "port", cfg.HTTPPort, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed to start", "err", err)
			os.Exit(1)
		}
	}()

	waitForShutdown(srv, log, cfg.HTTPShutdownTimeout)
}

func newRouter(log *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.HandleMethodNotAllowed = true

	r.Use(httpx.RequestID(), httpx.Logging(log), httpx.Recovery(log))

	r.NoRoute(func(c *gin.Context) {
		httpx.RespondError(c, http.StatusNotFound, "not_found", "Route tidak ditemukan.")
	})
	r.NoMethod(func(c *gin.Context) {
		httpx.RespondError(c, http.StatusMethodNotAllowed, "method_not_allowed", "Method tidak diizinkan.")
	})

	// Health is deliberately unversioned (no /v1 prefix) — it is
	// infrastructure consumed by orchestrators, not a product API
	// endpoint. See docs/architecture/api.md.
	r.GET("/health", func(c *gin.Context) {
		httpx.OK(c, http.StatusOK, gin.H{"status": "ok", "version": "dev"})
	})

	return r
}

func waitForShutdown(srv *http.Server, log *slog.Logger, timeout time.Duration) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}

	log.Info("shutdown complete")
}
