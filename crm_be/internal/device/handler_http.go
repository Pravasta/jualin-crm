package device

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

type Handler struct {
	usecase *Usecase
}

func NewHandler(usecase *Usecase) *Handler {
	return &Handler{usecase: usecase}
}

// RegisterRoutes mounts /v1/device-tokens, entirely behind authMW —
// principal user only (Phase 5 TD §9.1: every role, Owner through
// Employee, not just Employee — no reason to shut the door on an Owner
// who eventually installs the app too).
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/device-tokens")
	g.Use(authMW)
	g.POST("", h.register)
	g.DELETE("", h.unregister)
}

type registerRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (h *Handler) register(c *gin.Context) {
	t := authn.TenantFromContext(c)

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	tok, err := h.usecase.Register(c.Request.Context(), t, RegisterInput{Token: req.Token, Platform: req.Platform})
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, deviceTokenJSON(tok))
}

// unregisterRequest — DELETE with a JSON body rather than
// /v1/device-tokens/:token: an FCM token can legally contain characters
// that need escaping in a path segment, and there's exactly one query
// this endpoint ever needs, not a resource collection addressed by id.
type unregisterRequest struct {
	Token string `json:"token"`
}

func (h *Handler) unregister(c *gin.Context) {
	t := authn.TenantFromContext(c)

	var req unregisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	if err := h.usecase.Unregister(c.Request.Context(), t, req.Token); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// deviceTokenJSON never includes the raw token — the client already has
// it (it just sent it), and echoing it back in every response is a
// second place for it to leak into a log capturing HTTP bodies (Rule
// #26, same reasoning apiKeyJSON follows for secret_hash).
func deviceTokenJSON(tok *Token) gin.H {
	return gin.H{
		"id":            tok.ID,
		"membership_id": tok.MembershipID,
		"platform":      tok.Platform,
		"created_at":    tok.CreatedAt,
		"last_seen_at":  tok.LastSeenAt,
	}
}
