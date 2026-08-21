package invitation

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/ratelimit"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// createLimit/createWindow are deliberately conservative defaults, same
// reasoning as auth's rate limit constants — freeze leaves final numbers
// open until real traffic exists to tune against.
const (
	createLimit  = 10
	createWindow = time.Hour
)

type Handler struct {
	usecase       *Usecase
	createLimiter ratelimit.Limiter
}

func NewHandler(usecase *Usecase) *Handler {
	return &Handler{
		usecase:       usecase,
		createLimiter: ratelimit.NewFixedWindow(createLimit, createWindow),
	}
}

// RegisterRoutes mounts /v1/invitations. Create/List/Revoke sit behind
// authMW; token-info and accept are public/optionally-authenticated —
// TD phase 1 §6.1 explicitly calls accept "publik atau terautentikasi".
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW, optionalAuthMW gin.HandlerFunc) {
	protected := r.Group("/v1/invitations")
	protected.Use(authMW)
	protected.POST("", h.create)
	protected.GET("", h.list)
	protected.DELETE("/:id", h.revoke)

	public := r.Group("/v1/invitations")
	public.GET("/token/:token", h.tokenInfo)

	acceptGroup := r.Group("/v1/invitations")
	acceptGroup.Use(optionalAuthMW)
	acceptGroup.POST("/accept", h.accept)
}

type createRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// create is rate-limited per organization (TD §11) — an org that's
// already authenticated could otherwise be used to spam arbitrary email
// addresses with invitation mail.
func (h *Handler) create(c *gin.Context) {
	t := authn.TenantFromContext(c)

	if !h.createLimiter.Allow("invite:org:" + t.OrganizationID.String()) {
		httpx.RespondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak undangan dalam periode ini.")
		return
	}

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	inv, err := h.usecase.Create(c.Request.Context(), t, CreateInput{Email: req.Email, Role: tenant.Role(req.Role)})
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	httpx.OK(c, http.StatusCreated, gin.H{
		"id":         inv.ID,
		"email":      inv.Email,
		"role":       inv.Role,
		"expires_at": inv.ExpiresAt,
	})
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	invites, err := h.usecase.List(c.Request.Context(), t)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(invites))
	for _, inv := range invites {
		out = append(out, gin.H{
			"id":         inv.ID,
			"email":      inv.Email,
			"role":       inv.Role,
			"expires_at": inv.ExpiresAt,
			"created_at": inv.CreatedAt,
		})
	}
	httpx.OK(c, http.StatusOK, out)
}

func (h *Handler) revoke(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	if err := h.usecase.Revoke(c.Request.Context(), t, id); err != nil {
		httpx.WriteError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) tokenInfo(c *gin.Context) {
	info, err := h.usecase.TokenInfo(c.Request.Context(), c.Param("token"))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	httpx.OK(c, http.StatusOK, gin.H{
		"organization_name": info.OrganizationName,
		"email":             info.Email,
		"user_exists":       info.UserExists,
	})
}

type acceptRequest struct {
	Token    string `json:"token"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

// accept reads whichever tenant.Context OptionalMiddleware may have
// populated — nil if the caller sent no (or an invalid) token, which is
// exactly the "branch 1: new user" case's expected shape. Usecase.Accept
// resolves the actual branch from the invitation's email, not from
// whether this request happens to be authenticated.
func (h *Handler) accept(c *gin.Context) {
	var req acceptRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "token", Code: "required"}))
		return
	}

	var authenticated *tenant.Context
	if t, ok := authn.TenantFromContextOK(c); ok {
		authenticated = &t
	}

	out, err := h.usecase.Accept(c.Request.Context(), authenticated, req.Token, req.FullName, req.Password)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	httpx.OK(c, http.StatusOK, gin.H{
		"user_id":         out.UserID,
		"membership_id":   out.MembershipID,
		"organization_id": out.OrganizationID,
	})
}
