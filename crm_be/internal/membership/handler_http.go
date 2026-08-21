package membership

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type Handler struct {
	usecase *Usecase
}

func NewHandler(usecase *Usecase) *Handler {
	return &Handler{usecase: usecase}
}

// RegisterRoutes mounts /v1/memberships behind authMW — built once at
// the composition root and shared across every domain (internal/shared/authn).
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/memberships")
	g.Use(authMW)
	g.GET("", h.list)
	g.PATCH("/:id", h.updateRole)
	g.DELETE("/:id", h.deactivate)
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	members, err := h.usecase.List(c.Request.Context(), t)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(members))
	for _, m := range members {
		out = append(out, gin.H{
			"id":         m.ID,
			"user_id":    m.UserID,
			"email":      m.Email,
			"full_name":  m.FullName,
			"role":       m.Role,
			"created_at": m.CreatedAt,
		})
	}
	httpx.OK(c, http.StatusOK, out)
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

func (h *Handler) updateRole(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil || !isValidRole(req.Role) {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "role", Code: "invalid_value"}))
		return
	}

	in := UpdateRoleInput{MembershipID: id, NewRole: tenant.Role(req.Role)}
	if err := h.usecase.UpdateRole(c.Request.Context(), t, in); err != nil {
		httpx.WriteError(c, err)
		return
	}

	httpx.OK(c, http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) deactivate(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	if err := h.usecase.Deactivate(c.Request.Context(), t, id); err != nil {
		httpx.WriteError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func isValidRole(role string) bool {
	switch tenant.Role(role) {
	case tenant.RoleOwner, tenant.RoleAdmin, tenant.RoleManager, tenant.RoleEmployee:
		return true
	default:
		return false
	}
}
