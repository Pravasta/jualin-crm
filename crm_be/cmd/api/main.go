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
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
	"github.com/Pravasta/jualin-crm/crm_be/internal/customer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/invitation"
	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/metrics"
	"github.com/Pravasta/jualin-crm/crm_be/internal/notification"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/logger"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/mailer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/task"
)

const readyPingTimeout = 2 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		// No logger exists yet at this point — config failure is reported
		// to stderr directly and the process exits immediately (TD §3).
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	log := logger.New(cfg)

	ctx := context.Background()
	pool, err := db.New(ctx, cfg)
	if err != nil {
		// Fail fast, consistent with config validation: an unreachable
		// database at boot should stop deployment immediately rather than
		// surface as a wall of failed requests. docker-compose's
		// depends_on: service_healthy keeps this from firing in normal
		// `docker compose up`.
		log.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	router := newRouter(log, pool, cfg)

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

func newRouter(log *slog.Logger, pool *pgxpool.Pool, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.HandleMethodNotAllowed = true

	r.Use(httpx.RequestID(), httpx.Logging(log), httpx.Recovery(log))
	// CORS runs before any route is registered and before authMW is built
	// below — a preflight OPTIONS request carries no credentials and must
	// never reach the auth layer (Phase 3 TD §1.2).
	r.Use(httpx.CORS(cfg.CORSAllowedOrigins))

	mail := newMailer(cfg, log)
	authStore := newAuthStore(pool)
	tokenCfg := auth.TokenConfig{
		JWTSecret:                []byte(cfg.JWTSecret),
		AccessTokenTTL:           cfg.AccessTokenTTL,
		RefreshTokenTTLDashboard: cfg.RefreshTokenTTLDashboard,
		RefreshTokenTTLMobile:    cfg.RefreshTokenTTLMobile,
	}
	authUsecase := auth.NewUsecase(authStore, mail, log, cfg.AppBaseURL, tokenCfg)
	cookieCfg := auth.CookieConfig{Domain: cfg.CookieDomain, Secure: cfg.CookieSecure}

	// authMW is built once here and shared across every domain's
	// protected routes (internal/shared/authn) — no domain package
	// imports internal/auth just to check who's logged in.
	authMW := authn.Middleware(authUsecase)
	optionalAuthMW := authn.OptionalMiddleware(authUsecase)

	auth.NewHandler(authUsecase, cookieCfg).RegisterRoutes(r, authMW)

	membershipUsecase := membership.NewUsecase(newMembershipStore(pool))
	membership.NewHandler(membershipUsecase).RegisterRoutes(r, authMW)

	invitationUsecase := invitation.NewUsecase(newInvitationStore(pool), mail, log, cfg.AppBaseURL)
	invitation.NewHandler(invitationUsecase).RegisterRoutes(r, authMW, optionalAuthMW)

	leadUsecase := lead.NewUsecase(newLeadStore(pool))
	lead.NewHandler(leadUsecase).RegisterRoutes(r, authMW)

	activityUsecase := activity.NewUsecase(newActivityStore(pool))
	activity.NewHandler(activityUsecase).RegisterRoutes(r, authMW)

	taskUsecase := task.NewUsecase(newTaskStore(pool))
	task.NewHandler(taskUsecase).RegisterRoutes(r, authMW)

	notificationUsecase := notification.NewUsecase(newNotificationStore(pool))
	notification.NewHandler(notificationUsecase).RegisterRoutes(r, authMW)

	customerUsecase := customer.NewUsecase(newCustomerStore(pool))
	customer.NewHandler(customerUsecase).RegisterRoutes(r, authMW)

	// metrics is read-only (no Store/InTx, TD phase 3 §2) — its
	// Repository is built directly from pool, no _store.go wrapper needed.
	metricsUsecase := metrics.NewUsecase(metrics.New(pool))
	metrics.NewHandler(metricsUsecase).RegisterRoutes(r, authMW)

	r.NoRoute(func(c *gin.Context) {
		httpx.RespondError(c, http.StatusNotFound, "not_found", "Route tidak ditemukan.")
	})
	r.NoMethod(func(c *gin.Context) {
		httpx.RespondError(c, http.StatusMethodNotAllowed, "method_not_allowed", "Method tidak diizinkan.")
	})

	// Health is deliberately unversioned (no /v1 prefix) — it is
	// infrastructure consumed by orchestrators, not a product API
	// endpoint. See docs/architecture/api.md.
	//
	// /health checks liveness only — it must never touch the database, or
	// a slow/dead DB would make the orchestrator kill a process that is
	// otherwise fine to keep running.
	r.GET("/health", func(c *gin.Context) {
		httpx.OK(c, http.StatusOK, gin.H{"status": "ok", "version": "dev"})
	})

	// /health/ready checks readiness by pinging the database on every
	// call — it reflects current connectivity, not just boot-time state,
	// so it correctly reports 503 if postgres goes down after a
	// successful start.
	r.GET("/health/ready", func(c *gin.Context) {
		if err := db.Ping(c.Request.Context(), pool, readyPingTimeout); err != nil {
			httpx.Logger(c).Warn("readiness check failed", "err", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"data": gin.H{"status": "degraded", "database": "unreachable"}})
			return
		}
		httpx.OK(c, http.StatusOK, gin.H{"status": "ok", "database": "reachable"})
	})

	return r
}

// newMailer picks the mailer implementation for cfg.MailProvider.
// "log" (LogMailer) is the only option today — config.validate rejects
// any other value, so the default case here is unreachable, not a
// silent fallback.
func newMailer(cfg *config.Config, log *slog.Logger) mailer.Mailer {
	switch cfg.MailProvider {
	case "log":
		return mailer.NewLogMailer(log)
	default:
		panic(fmt.Sprintf("unreachable: unknown MAIL_PROVIDER %q passed config validation", cfg.MailProvider))
	}
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
