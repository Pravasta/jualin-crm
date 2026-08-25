package metrics

import (
	"net/http"
	"time"

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

// RegisterRoutes mounts /v1/metrics behind authMW — built once at the
// composition root and shared across every domain (internal/shared/authn).
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/metrics")
	g.Use(authMW)
	g.GET("/summary", h.summary)
	g.GET("/employees", h.employees)
}

// parseFilter mirrors internal/lead's created_from/created_to parsing —
// an unparseable or absent value is silently ignored rather than
// rejected, leaving that bound unset (Filter's nil = unbounded).
func parseFilter(c *gin.Context) Filter {
	var f Filter
	if from, err := time.Parse(time.RFC3339, c.Query("from")); err == nil {
		f.From = &from
	}
	if to, err := time.Parse(time.RFC3339, c.Query("to")); err == nil {
		f.To = &to
	}
	return f
}

func (h *Handler) summary(c *gin.Context) {
	t := authn.TenantFromContext(c)

	s, err := h.usecase.Summary(c.Request.Context(), t, parseFilter(c))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, summaryJSON(s))
}

func (h *Handler) employees(c *gin.Context) {
	t := authn.TenantFromContext(c)

	out, err := h.usecase.Employees(c.Request.Context(), t, parseFilter(c))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	data := make([]gin.H, 0, len(out))
	for _, em := range out {
		data = append(data, employeeJSON(em))
	}
	httpx.OK(c, http.StatusOK, data)
}

func summaryJSON(s *Summary) gin.H {
	return gin.H{
		"total_new":       s.TotalNew,
		"by_status":       s.ByStatus,
		"unassigned":      s.Unassigned,
		"conversion_rate": s.ConversionRate,
	}
}

func employeeJSON(em *EmployeeMetric) gin.H {
	return gin.H{
		"membership_id":        em.MembershipID,
		"full_name":            em.FullName,
		"lead_count":           em.LeadCount,
		"avg_response_seconds": em.AvgResponseSeconds,
		"converted_count":      em.ConvertedCount,
	}
}
