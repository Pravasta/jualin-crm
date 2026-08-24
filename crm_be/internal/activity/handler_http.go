package activity

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

// RegisterRoutes mounts a SECOND group at /v1/leads — gin allows
// multiple independent groups sharing a path prefix as long as the
// final method+path pairs never collide. The wildcard segment MUST be
// named :id here, matching lead.Handler's own /v1/leads/:id exactly —
// gin panics at router-build time if two routes disagree on a
// wildcard's name at the same tree position.
//
// No PATCH or DELETE route exists anywhere in this file — append-only
// (TD §1.3/§8) is enforced by the router literally not having those
// routes, not by a convention someone has to remember.
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/leads")
	g.Use(authMW)
	g.GET("/:id/activities", h.list)
	g.POST("/:id/activities", h.create)
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	leadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	activities, err := h.usecase.List(c.Request.Context(), t, leadID)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(activities))
	for _, a := range activities {
		out = append(out, activityJSON(a))
	}
	httpx.OK(c, http.StatusOK, out)
}

type createRequest struct {
	Type     string         `json:"type"`
	Body     *string        `json:"body"`
	Metadata map[string]any `json:"metadata"`
}

func (h *Handler) create(c *gin.Context) {
	t := authn.TenantFromContext(c)

	leadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	created, err := h.usecase.Create(c.Request.Context(), t, leadID, CreateActivityInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, activityJSON(created))
}

func activityJSON(a *Activity) gin.H {
	return gin.H{
		"id":                  a.ID,
		"lead_id":             a.LeadID,
		"type":                a.Type,
		"actor_membership_id": a.ActorMembershipID,
		"body":                a.Body,
		"metadata":            a.Metadata,
		"created_at":          a.CreatedAt,
	}
}
