package customer

import (
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

// RegisterRoutes mounts a SECOND group at /v1/leads (same :id wildcard
// discipline activity/task already established there) for the convert
// action, plus /v1/customers for plain CRUD.
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	leads := r.Group("/v1/leads")
	leads.Use(authMW)
	leads.POST("/:id/convert", h.convert)

	customers := r.Group("/v1/customers")
	customers.Use(authMW)
	customers.GET("", h.list)
	customers.GET("/:id", h.get)
	customers.PATCH("/:id", h.update)
	customers.DELETE("/:id", h.delete)
}

func (h *Handler) convert(c *gin.Context) {
	t := authn.TenantFromContext(c)

	leadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	created, err := h.usecase.Convert(c.Request.Context(), t, leadID)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, customerJSON(created))
}

func (h *Handler) get(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	found, err := h.usecase.Get(c.Request.Context(), t, id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, customerJSON(found))
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	in := ListInput{Query: c.Query("q")}
	if page, err := strconv.Atoi(c.Query("page")); err == nil {
		in.Page = page
	}
	if perPage, err := strconv.Atoi(c.Query("per_page")); err == nil {
		in.PerPage = perPage
	}

	customers, meta, err := h.usecase.List(c.Request.Context(), t, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(customers))
	for _, cu := range customers {
		out = append(out, customerJSON(cu))
	}
	httpx.List(c, out, meta)
}

type updateRequest struct {
	Name    *string `json:"name"`
	Email   *string `json:"email"`
	Phone   *string `json:"phone"`
	Company *string `json:"company"`
	Notes   *string `json:"notes"`
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

	updated, err := h.usecase.Update(c.Request.Context(), t, id, UpdateCustomerInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, customerJSON(updated))
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

func customerJSON(cu *Customer) gin.H {
	return gin.H{
		"id":                         cu.ID,
		"name":                       cu.Name,
		"email":                      cu.Email,
		"phone":                      cu.Phone,
		"phone_e164":                 cu.PhoneE164,
		"company":                    cu.Company,
		"notes":                      cu.Notes,
		"converted_from_lead_id":     cu.ConvertedFromLeadID,
		"converted_by_membership_id": cu.ConvertedByMembershipID,
		"converted_at":               cu.ConvertedAt,
		"created_at":                 cu.CreatedAt,
		"updated_at":                 cu.UpdatedAt,
	}
}
