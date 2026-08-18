package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/ratelimit"
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
)

// Handler wires HTTP to Service. Rate limiting is checked here rather
// than in Service: the "per email" dimension for resend needs the
// parsed request body, which only the handler has — Service's job is
// business logic, not request shaping.
type Handler struct {
	service *Service

	registerLimiter    ratelimit.Limiter
	resendEmailLimiter ratelimit.Limiter
	resendIPLimiter    ratelimit.Limiter
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service:            service,
		registerLimiter:    ratelimit.NewFixedWindow(registerLimit, registerWindow),
		resendEmailLimiter: ratelimit.NewFixedWindow(resendLimit, resendWindow),
		resendIPLimiter:    ratelimit.NewFixedWindow(resendIPLimit, resendIPWindow),
	}
}

// RegisterRoutes mounts every /v1/auth/* endpoint this issue implements.
// Login, refresh, and password endpoints are added in #10.
func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/v1/auth")
	g.POST("/register", h.register)
	g.POST("/verify-email", h.verifyEmail)
	g.POST("/verify-email/resend", h.resendVerification)
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

	out, err := h.service.Register(c.Request.Context(), RegisterInput(req))
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

	if err := h.service.VerifyEmail(c.Request.Context(), req.Token); err != nil {
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

	h.service.ResendVerification(c.Request.Context(), req.Email)

	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": "accepted"}})
}
