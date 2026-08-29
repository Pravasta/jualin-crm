package apikey

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

type Handler struct {
	usecase *Usecase
}

func NewHandler(usecase *Usecase) *Handler {
	return &Handler{usecase: usecase}
}

// RegisterRoutes mounts /v1/api-keys, entirely behind authMW — every
// route here is principal user only (Owner/Admin, enforced in authz).
// The credential path this issue's keys will eventually authenticate
// (POST /v1/leads) is #47's, not mounted from this package.
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/api-keys")
	g.Use(authMW)
	g.POST("", h.create)
	g.GET("", h.list)
	g.DELETE("/:id", h.revoke)
}

type createRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// create is the ONE response in this entire API that carries a raw
// secret (Rule #21) — apiKeyJSON never includes it, so this handler adds
// "secret" onto that JSON explicitly rather than apiKeyJSON gaining an
// optional field some caller might forget to omit.
func (h *Handler) create(c *gin.Context) {
	t := authn.TenantFromContext(c)

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	k, raw, err := h.usecase.Create(c.Request.Context(), t, CreateInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	body := apiKeyJSON(k)
	body["secret"] = raw
	httpx.OK(c, http.StatusCreated, body)
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	keys, err := h.usecase.List(c.Request.Context(), t)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(keys))
	for _, k := range keys {
		out = append(out, apiKeyJSON(k))
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

// apiKeyJSON never includes secret_hash or a raw secret — the only place
// a raw secret appears is create's response, assembled separately above.
func apiKeyJSON(k *APIKey) gin.H {
	return gin.H{
		"id":                       k.ID,
		"key_prefix":               k.KeyPrefix,
		"name":                     k.Name,
		"scopes":                   k.Scopes,
		"created_by_membership_id": k.CreatedByMembershipID,
		"created_at":               k.CreatedAt,
		"last_used_at":             k.LastUsedAt,
		"revoked_at":               k.RevokedAt,
		"expires_at":               k.ExpiresAt,
	}
}
