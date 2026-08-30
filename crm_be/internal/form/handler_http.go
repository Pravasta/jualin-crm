package form

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

// RegisterRoutes mounts /v1/forms, entirely behind authMW — every route
// here is principal user only (Owner/Admin, enforced in authz). The
// public POST /v1/forms/{public_key}/submit path this issue's forms
// will eventually accept is #87's, not mounted from this package; the
// GET /embed/{public_key} page is #88's, mounted outside the /v1
// namespace entirely (TD §8).
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/forms")
	g.Use(authMW)
	g.POST("", h.create)
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.delete)
}

type createRequest struct {
	Name   string  `json:"name"`
	Fields *Fields `json:"fields"`
}

func (h *Handler) create(c *gin.Context) {
	t := authn.TenantFromContext(c)

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	f, err := h.usecase.Create(c.Request.Context(), t, CreateInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, formJSON(f))
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	forms, err := h.usecase.List(c.Request.Context(), t)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(forms))
	for _, f := range forms {
		out = append(out, formJSON(f))
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

	f, err := h.usecase.Get(c.Request.Context(), t, id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, formJSON(f))
}

type updateRequest struct {
	Name           *string   `json:"name"`
	Fields         *Fields   `json:"fields"`
	AllowedOrigins *[]string `json:"allowed_origins"`
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

	f, err := h.usecase.Update(c.Request.Context(), t, id, UpdateInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, formJSON(f))
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

// formJSON always includes public_key in full — unlike apiKeyJSON,
// which never includes a raw secret, public_key is not a secret at all
// (D3) and is exactly the value the owner needs to copy into the embed
// snippet every time this form is viewed, not just once at creation.
func formJSON(f *Form) gin.H {
	return gin.H{
		"id":                       f.ID,
		"public_key":               f.PublicKey,
		"name":                     f.Name,
		"fields":                   f.Fields,
		"allowed_origins":          f.AllowedOrigins,
		"submit_count":             f.SubmitCount,
		"created_by_membership_id": f.CreatedByMembershipID,
		"created_at":               f.CreatedAt,
		"updated_at":               f.UpdatedAt,
	}
}
