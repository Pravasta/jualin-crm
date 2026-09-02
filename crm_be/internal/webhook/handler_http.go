package webhook

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// RegisterRoutes mounts /v1/webhook-endpoints and /v1/webhook-deliveries,
// entirely behind authMW — every route is principal user only (Owner/
// Admin, enforced in authz). The path is webhook-endpoints, not webhooks,
// so /v1/webhook-deliveries/:id/retry sits beside it without a wildcard
// collision (TD §6).
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/webhook-endpoints")
	g.Use(authMW)
	g.POST("", h.create)
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.delete)
	g.GET("/:id/deliveries", h.listDeliveries)

	d := r.Group("/v1/webhook-deliveries")
	d.Use(authMW)
	d.POST("/:id/retry", h.retryDelivery)
}

type createRequest struct {
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Description string   `json:"description"`
}

// create is the ONE response here that carries a raw secret (Rule #21) —
// endpointJSON never includes it, so this handler adds "secret" onto that
// JSON explicitly, the same shape as apikey's create.
func (h *Handler) create(c *gin.Context) {
	t := authn.TenantFromContext(c)

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	e, secret, err := h.usecase.Create(c.Request.Context(), t, CreateInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	body := endpointJSON(e)
	body["secret"] = secret
	httpx.OK(c, http.StatusCreated, body)
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	endpoints, err := h.usecase.List(c.Request.Context(), t)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(endpoints))
	for _, e := range endpoints {
		out = append(out, endpointJSON(e))
	}
	httpx.OK(c, http.StatusOK, out)
}

func (h *Handler) get(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	e, err := h.usecase.Get(c.Request.Context(), t, id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, endpointJSON(e))
}

type updateRequest struct {
	URL         *string   `json:"url"`
	Events      *[]string `json:"events"`
	Description *string   `json:"description"`
	IsActive    *bool     `json:"is_active"`
}

func (h *Handler) update(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	e, err := h.usecase.Update(c.Request.Context(), t, id, UpdateInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, endpointJSON(e))
}

func (h *Handler) delete(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	if err := h.usecase.Delete(c.Request.Context(), t, id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listDeliveries(c *gin.Context) {
	t := authn.TenantFromContext(c)

	endpointID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}

	deliveries, total, err := h.usecase.ListDeliveries(c.Request.Context(), t, endpointID, page)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(deliveries))
	for _, d := range deliveries {
		out = append(out, deliveryJSON(d))
	}
	httpx.List(c, out, httpx.Meta{Page: page, PerPage: deliveryHistoryPageSize, Total: total})
}

func (h *Handler) retryDelivery(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	d, err := h.usecase.RetryDelivery(c.Request.Context(), t, id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, deliveryJSON(d))
}

// endpointJSON never includes secret_hash or a raw secret — the only
// place a raw secret appears is create's response, assembled separately.
// secret_prefix IS included: it's the non-sensitive display fragment
// (like api_keys' key_prefix).
func endpointJSON(e *Endpoint) gin.H {
	return gin.H{
		"id":                       e.ID,
		"url":                      e.URL,
		"secret_prefix":            e.SecretPrefix,
		"events":                   e.Events,
		"description":              e.Description,
		"is_active":                e.IsActive,
		"created_by_membership_id": e.CreatedByMembershipID,
		"created_at":               e.CreatedAt,
		"updated_at":               e.UpdatedAt,
	}
}

// deliveryJSON is the history row shape the dashboard renders (#103) —
// payload is included so an Owner can see exactly what was sent;
// delivering_since is internal worker bookkeeping and stays out.
func deliveryJSON(d *Delivery) gin.H {
	return gin.H{
		"id":              d.ID,
		"endpoint_id":     d.EndpointID,
		"event_type":      d.EventType,
		"payload":         payloadJSON(d.Payload),
		"status":          d.Status,
		"attempt":         d.Attempt,
		"next_attempt_at": d.NextAttemptAt,
		"response_status": d.ResponseStatus,
		"error":           d.Error,
		"created_at":      d.CreatedAt,
		"updated_at":      d.UpdatedAt,
	}
}

// payloadJSON embeds the stored jsonb payload as real JSON in the
// response, not a base64 string — but degrades to null for an
// empty/invalid slice rather than emitting a broken document.
func payloadJSON(b []byte) any {
	if !json.Valid(b) {
		return nil
	}
	return json.RawMessage(b)
}
