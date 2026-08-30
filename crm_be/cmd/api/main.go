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
	"github.com/Pravasta/jualin-crm/crm_be/internal/apikey"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
	"github.com/Pravasta/jualin-crm/crm_be/internal/customer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/device"
	"github.com/Pravasta/jualin-crm/crm_be/internal/form"
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
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/push"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/ratelimit"
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

	// Must run before any middleware that reads c.ClientIP() — Logging
	// (below) is one, and every rate limiter keyed by IP is another
	// (Phase 4.5 TD §1). config.Validate already proved every entry
	// parses as an IP/CIDR, so the only remaining failure mode here is
	// this build's Gin disagreeing with net.ParseIP/ParseCIDR — a bug,
	// not a config problem, and ADR-010 says boot must not proceed
	// half-ready.
	if err := r.SetTrustedProxies(cfg.TrustedProxyCIDRs()); err != nil {
		panic(fmt.Sprintf("trusted proxies rejected by gin despite passing config validation: %v", err))
	}

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

	// apikey is wired here, ahead of lead, because lead's public create
	// route (below) needs apikeyUsecase to build its own middleware —
	// apikeyUsecase.ResolveAPIKey structurally satisfies
	// authn.APIKeyResolver (Phase 4 #47, TD §3). The management routes
	// (create/list/revoke as principal user) are registered later,
	// alongside every other domain's own handler.
	apikeyUsecase := apikey.NewUsecase(newAPIKeyStore(pool))
	publicLeadCreateMW := authn.MiddlewareWithAPIKey(authUsecase, apikeyUsecase)

	// device is wired here, ahead of lead, for the same reason apikey is
	// above it: lead.PushSender (bridged below) needs deviceUsecase to
	// already exist. deviceUsecase.PushToMembership structurally
	// satisfies lead.PushSender (Phase 5 #68, TD §9.3) — internal/lead
	// never imports this package, same bridging pattern as
	// NotificationSender/ActivityRecorder.
	deviceUsecase := device.NewUsecase(newDeviceStore(pool), newPushSender(cfg, log), log)

	leadUsecase := lead.NewUsecase(newLeadStore(pool, deviceUsecase))
	leadRateLimiter := ratelimit.NewFixedWindow(cfg.PublicAPIRateLimit, time.Minute)
	lead.NewHandler(leadUsecase, leadRateLimiter).RegisterRoutes(r, authMW, publicLeadCreateMW)

	activityUsecase := activity.NewUsecase(newActivityStore(pool))
	activity.NewHandler(activityUsecase).RegisterRoutes(r, authMW)

	taskUsecase := task.NewUsecase(newTaskStore(pool))
	task.NewHandler(taskUsecase).RegisterRoutes(r, authMW)

	notificationUsecase := notification.NewUsecase(newNotificationStore(pool))
	notification.NewHandler(notificationUsecase).RegisterRoutes(r, authMW)

	customerUsecase := customer.NewUsecase(newCustomerStore(pool))
	customer.NewHandler(customerUsecase).RegisterRoutes(r, authMW)

	// apikey's own management routes (create/list/revoke as principal
	// user) — apikeyUsecase itself was built earlier, alongside lead's
	// wiring, which is the only thing that needed it ahead of this point.
	apikey.NewHandler(apikeyUsecase).RegisterRoutes(r, authMW)

	// device's own management routes (register/unregister as principal
	// user) — deviceUsecase itself was built earlier, alongside lead's
	// wiring, which is the only thing that needed it ahead of this point.
	device.NewHandler(deviceUsecase).RegisterRoutes(r, authMW)

	// metrics is read-only (no Store/InTx, TD phase 3 §2) — its
	// Repository is built directly from pool, no _store.go wrapper needed.
	metricsUsecase := metrics.NewUsecase(metrics.New(pool))
	metrics.NewHandler(metricsUsecase).RegisterRoutes(r, authMW)

	// form (Phase 6 #85) is management-only in this issue — no public
	// route needs formUsecase ahead of time the way lead needed apikey/
	// device, so it's wired here alongside every other domain's own
	// handler rather than earlier. The public POST /v1/forms/{public_key}/
	// submit route and GET /embed/{public_key} page are #87/#88's, not
	// mounted from this package yet.
	formUsecase := form.NewUsecase(newFormStore(pool))
	form.NewHandler(formUsecase).RegisterRoutes(r, authMW)

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
// config.validate rejects any value outside {"log", "smtp"} and rejects
// "log" outright when APP_ENV=production (Phase 4.6 decision E2), so the
// default case here stays unreachable, not a silent fallback.
func newMailer(cfg *config.Config, log *slog.Logger) mailer.Mailer {
	switch cfg.MailProvider {
	case "log":
		return mailer.NewLogMailer(log)
	case "smtp":
		return mailer.NewSMTPMailer(mailer.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.MailFrom,
			TLS:      cfg.SMTPTLS,
			Timeout:  cfg.SMTPTimeout,
		})
	default:
		panic(fmt.Sprintf("unreachable: unknown MAIL_PROVIDER %q passed config validation", cfg.MailProvider))
	}
}

// newPushSender picks the push implementation for cfg.PushProvider — same
// shape as newMailer (Phase 5 #68). Unlike newMailer's two branches,
// push.NewFCMSender can genuinely fail (a missing or malformed
// credentials file is a real, reachable misconfiguration, not a code
// invariant) — panicking here rather than threading an error through
// newRouter's signature still satisfies ADR-010: this runs synchronously
// during router construction, before ListenAndServe is ever called and
// before httpx.Recovery's request-scoped middleware exists to catch
// anything, so the process exits nonzero before it could serve a single
// request half-configured.
func newPushSender(cfg *config.Config, log *slog.Logger) push.Sender {
	switch cfg.PushProvider {
	case "none":
		return push.NewNoopSender(log)
	case "fcm":
		sender, err := push.NewFCMSender(push.FCMConfig{
			ProjectID:       cfg.FCMProjectID,
			CredentialsFile: cfg.FCMCredentialsFile,
			Timeout:         cfg.PushTimeout,
		})
		if err != nil {
			panic(fmt.Sprintf("push: failed to construct FCM sender from FCM_CREDENTIALS_FILE=%q: %v", cfg.FCMCredentialsFile, err))
		}
		return sender
	default:
		panic(fmt.Sprintf("unreachable: unknown PUSH_PROVIDER %q passed config validation", cfg.PushProvider))
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
