package auth

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/ratelimit"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/token"
)

// Rate limit figures are deliberately conservative defaults, not final
// numbers — freeze leaves "strategi rate limit final" open until real
// traffic exists to tune against (docs/STATUS.md). What matters now is
// that every email-sending endpoint has *some* limit (Rule #34), not the
// exact figure.
const (
	registerLimit  = 5
	registerWindow = time.Hour
	resendLimit    = 3
	resendWindow   = time.Hour
	resendIPLimit  = 10
	resendIPWindow = time.Hour
	forgotLimit    = 3
	forgotWindow   = time.Hour
	forgotIPLimit  = 10
	forgotIPWindow = time.Hour
)

// CookieConfig is the subset of config.Config the HTTP layer needs to
// set cookies correctly — passed explicitly rather than the whole
// config.Config so Handler's dependencies stay legible from its
// constructor signature alone.
type CookieConfig struct {
	Domain string
	Secure bool
}

// Handler wires HTTP to Usecase. Rate limiting is checked here rather
// than in Usecase: the "per email"/"per IP" dimensions need the parsed
// request body and client IP, which only the handler has — Usecase's
// job is business logic, not request shaping.
type Handler struct {
	usecase *Usecase
	cookies CookieConfig

	registerLimiter    ratelimit.Limiter
	resendEmailLimiter ratelimit.Limiter
	resendIPLimiter    ratelimit.Limiter
	forgotEmailLimiter ratelimit.Limiter
	forgotIPLimiter    ratelimit.Limiter
	loginLimiter       *LoginLimiter
}

func NewHandler(usecase *Usecase, cookies CookieConfig) *Handler {
	return &Handler{
		usecase:            usecase,
		cookies:            cookies,
		registerLimiter:    ratelimit.NewFixedWindow(registerLimit, registerWindow),
		resendEmailLimiter: ratelimit.NewFixedWindow(resendLimit, resendWindow),
		resendIPLimiter:    ratelimit.NewFixedWindow(resendIPLimit, resendIPWindow),
		forgotEmailLimiter: ratelimit.NewFixedWindow(forgotLimit, forgotWindow),
		forgotIPLimiter:    ratelimit.NewFixedWindow(forgotIPLimit, forgotIPWindow),
		loginLimiter:       NewLoginLimiter(),
	}
}

// RegisterRoutes mounts every /v1/auth/* endpoint plus GET /v1/me.
func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/v1/auth")
	g.POST("/register", h.register)
	g.POST("/verify-email", h.verifyEmail)
	g.POST("/verify-email/resend", h.resendVerification)
	g.POST("/login", h.login)
	g.POST("/refresh", h.refresh)
	g.POST("/logout", h.logout)
	g.POST("/password/forgot", h.forgotPassword)
	g.POST("/password/reset", h.resetPassword)

	protected := r.Group("/v1")
	protected.Use(AuthMiddleware(h.usecase))
	protected.GET("/me", h.me)
}

type registerRequest struct {
	OrganizationName string `json:"organization_name"`
	FullName         string `json:"full_name"`
	Email            string `json:"email"`
	Password         string `json:"password"`
}

func (h *Handler) register(c *gin.Context) {
	if !h.registerLimiter.Allow("ip:" + c.ClientIP()) {
		httpx.RespondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan. Coba lagi nanti.")
		return
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	out, err := h.usecase.Register(c.Request.Context(), RegisterInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	httpx.OK(c, http.StatusCreated, gin.H{
		"user_id":         out.UserID,
		"organization_id": out.OrganizationID,
	})
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

func (h *Handler) verifyEmail(c *gin.Context) {
	var req verifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		httpx.WriteError(c, invalidTokenError())
		return
	}

	if err := h.usecase.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		httpx.WriteError(c, err)
		return
	}

	httpx.OK(c, http.StatusOK, gin.H{"status": "verified"})
}

type resendVerificationRequest struct {
	Email string `json:"email"`
}

// resendVerification always answers 202, whether or not the email exists
// or is already verified (TD §6.3, anti-enumeration).
func (h *Handler) resendVerification(c *gin.Context) {
	var req resendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "email", Code: "required"}))
		return
	}

	if !h.resendEmailLimiter.Allow("email:"+req.Email) || !h.resendIPLimiter.Allow("ip:"+c.ClientIP()) {
		httpx.RespondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan. Coba lagi nanti.")
		return
	}

	h.usecase.ResendVerification(c.Request.Context(), req.Email)

	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": "accepted"}})
}

type loginRequest struct {
	Email          string  `json:"email"`
	Password       string  `json:"password"`
	Client         string  `json:"client"`
	OrganizationID *string `json:"organization_id"`
}

// login rejects on the login-specific progressive-backoff limiter
// (TD §11) *before* touching the usecase — an IP or email already in
// backoff shouldn't even pay for a password hash comparison. A wrong
// login records a failure and, if the limiter is now blocking, still
// let this response go out (429 only kicks in on the *next* attempt) —
// TD doesn't ask for a 429 on the request that itself just tripped the
// limiter, only on subsequent ones.
func (h *Handler) login(c *gin.Context) {
	ip := "login:ip:" + c.ClientIP()

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}
	emailKey := "login:email:" + req.Email

	if ok, retryAfter := h.loginLimiter.Allow(ip); !ok {
		respondRateLimited(c, retryAfter)
		return
	}
	if ok, retryAfter := h.loginLimiter.Allow(emailKey); !ok {
		respondRateLimited(c, retryAfter)
		return
	}

	in := LoginInput{Email: req.Email, Password: req.Password, Client: req.Client}
	if req.OrganizationID != nil {
		if id, err := uuid.Parse(*req.OrganizationID); err == nil {
			in.OrganizationID = &id
		}
	}

	out, err := h.usecase.Login(c.Request.Context(), in)
	if err != nil {
		var selErr *OrganizationSelectionError
		if errors.As(err, &selErr) {
			respondOrganizationSelectionRequired(c, selErr)
			return
		}
		h.loginLimiter.RecordFailure(ip)
		h.loginLimiter.RecordFailure(emailKey)
		httpx.WriteError(c, err)
		return
	}
	h.loginLimiter.RecordSuccess(ip)
	h.loginLimiter.RecordSuccess(emailKey)

	h.respondSession(c, http.StatusOK, out.Client, out.AccessToken, out.RefreshToken, out.AccessTokenTTL, out.RefreshTokenTTL)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// refresh reads the refresh token from the cookie (dashboard) if present,
// falling back to the body (mobile) — matching login's client-specific
// transport (TD §5). CSRF only applies when the cookie was the source:
// a mobile client's body-supplied token was never sent automatically by
// a browser, so there's no ambient credential for a cross-site request
// to ride along on.
func (h *Handler) refresh(c *gin.Context) {
	raw, viaCookie := h.readRefreshToken(c)
	if raw == "" {
		httpx.WriteError(c, invalidCredentialsError())
		return
	}
	if viaCookie && !httpx.VerifyCSRF(c) {
		httpx.RespondError(c, http.StatusForbidden, "csrf_token_invalid", "Token CSRF tidak valid.")
		return
	}

	out, err := h.usecase.Refresh(c.Request.Context(), raw)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	h.respondSession(c, http.StatusOK, out.Client, out.AccessToken, out.RefreshToken, out.AccessTokenTTL, out.RefreshTokenTTL)
}

// logout always answers 204 — see Usecase.Logout's not-found-is-success
// reasoning. Cookies are cleared unconditionally for a cookie-sourced
// request regardless of what the usecase found.
func (h *Handler) logout(c *gin.Context) {
	raw, viaCookie := h.readRefreshToken(c)
	if viaCookie && !httpx.VerifyCSRF(c) {
		httpx.RespondError(c, http.StatusForbidden, "csrf_token_invalid", "Token CSRF tidak valid.")
		return
	}

	if raw != "" {
		if err := h.usecase.Logout(c.Request.Context(), raw); err != nil {
			httpx.WriteError(c, err)
			return
		}
	}
	if viaCookie {
		h.clearAuthCookies(c)
	}
	c.Status(http.StatusNoContent)
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// forgotPassword always answers 202 (TD §6.3, anti-enumeration) — same
// shape as resendVerification.
func (h *Handler) forgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "email", Code: "required"}))
		return
	}

	if !h.forgotEmailLimiter.Allow("forgot:email:"+req.Email) || !h.forgotIPLimiter.Allow("forgot:ip:"+c.ClientIP()) {
		httpx.RespondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan. Coba lagi nanti.")
		return
	}

	h.usecase.ForgotPassword(c.Request.Context(), req.Email)

	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": "accepted"}})
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) resetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		httpx.WriteError(c, invalidTokenError())
		return
	}

	if err := h.usecase.ResetPassword(c.Request.Context(), req.Token, req.Password); err != nil {
		httpx.WriteError(c, err)
		return
	}

	httpx.OK(c, http.StatusOK, gin.H{"status": "reset"})
}

func (h *Handler) me(c *gin.Context) {
	t := TenantFromContext(c)

	out, err := h.usecase.Me(c.Request.Context(), t)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	httpx.OK(c, http.StatusOK, gin.H{
		"user_id":           out.UserID,
		"email":             out.Email,
		"full_name":         out.FullName,
		"organization_id":   out.OrganizationID,
		"organization_name": out.OrganizationName,
		"membership_id":     out.MembershipID,
		"role":              out.Role,
	})
}

// readRefreshToken prefers the refresh_token cookie (dashboard) over the
// request body (mobile) — mirrors extractAccessToken's bearer-over-cookie
// preference in middleware.go, applied to the opposite credential.
func (h *Handler) readRefreshToken(c *gin.Context) (raw string, viaCookie bool) {
	if cookie, err := c.Cookie(refreshTokenCookieName); err == nil && cookie != "" {
		return cookie, true
	}
	var req refreshRequest
	// Body may already have been consumed by ShouldBindJSON elsewhere in
	// this request's lifetime; refresh/logout each bind at most once, so
	// that's not a concern here.
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		return req.RefreshToken, false
	}
	return "", false
}

// respondSession writes the client-appropriate response: dashboard gets
// cookies and a body with no token fields at all; mobile gets a body
// with the tokens and no Set-Cookie header whatsoever (TD §5 acceptance
// criteria — verified structurally by which branch runs, not a flag).
func (h *Handler) respondSession(c *gin.Context, status int, client, accessToken, refreshToken string, accessTTL, refreshTTL time.Duration) {
	if client == ClientDashboard {
		h.setAuthCookies(c, accessToken, refreshToken, accessTTL, refreshTTL)
		httpx.OK(c, status, gin.H{"status": "ok"})
		return
	}
	httpx.OK(c, status, gin.H{
		"access_token":      accessToken,
		"refresh_token":     refreshToken,
		"access_token_ttl":  int(accessTTL.Seconds()),
		"refresh_token_ttl": int(refreshTTL.Seconds()),
	})
}

const refreshTokenCookieName = "refresh_token"

func (h *Handler) setAuthCookies(c *gin.Context, accessToken, refreshToken string, accessTTL, refreshTTL time.Duration) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(accessTokenCookieName, accessToken, int(accessTTL.Seconds()), "/", h.cookies.Domain, h.cookies.Secure, true)
	c.SetCookie(refreshTokenCookieName, refreshToken, int(refreshTTL.Seconds()), "/", h.cookies.Domain, h.cookies.Secure, true)

	// csrf_token is deliberately NOT HttpOnly — JavaScript must read it
	// to echo it back in X-CSRF-Token (httpx.VerifyCSRF).
	csrfRaw, _, err := token.Generate()
	if err == nil {
		c.SetCookie(httpx.CSRFCookieName, csrfRaw, int(refreshTTL.Seconds()), "/", h.cookies.Domain, h.cookies.Secure, false)
	}
}

func (h *Handler) clearAuthCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(accessTokenCookieName, "", -1, "/", h.cookies.Domain, h.cookies.Secure, true)
	c.SetCookie(refreshTokenCookieName, "", -1, "/", h.cookies.Domain, h.cookies.Secure, true)
	c.SetCookie(httpx.CSRFCookieName, "", -1, "/", h.cookies.Domain, h.cookies.Secure, false)
}

func respondRateLimited(c *gin.Context, retryAfter time.Duration) {
	c.Header("Retry-After", fmtSeconds(retryAfter))
	httpx.RespondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan. Coba lagi nanti.")
}

func fmtSeconds(d time.Duration) string {
	return strconv.Itoa(max(int(d.Seconds()), 1))
}

// respondOrganizationSelectionRequired writes TD §6.2's deliberate,
// documented extension of the error envelope — the only place in this
// codebase an error body carries a field beyond code/message/details.
func respondOrganizationSelectionRequired(c *gin.Context, selErr *OrganizationSelectionError) {
	orgs := make([]gin.H, 0, len(selErr.Organizations))
	for _, o := range selErr.Organizations {
		orgs = append(orgs, gin.H{"id": o.ID, "name": o.Name})
	}
	c.JSON(http.StatusConflict, gin.H{"error": gin.H{
		"code":          "organization_selection_required",
		"message":       "Pilih organization untuk melanjutkan.",
		"organizations": orgs,
	}})
}
