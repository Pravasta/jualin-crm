package metrics

import (
	"net/http"
	"slices"
	"time"

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

// RegisterRoutes mounts /v1/metrics behind authMW — built once at the
// composition root and shared across every domain (internal/shared/authn).
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/metrics")
	g.Use(authMW)
	g.GET("/summary", h.summary)
	g.GET("/employees", h.employees)
	g.GET("/trend", h.trend)
	g.GET("/sources", h.sources)
	g.GET("/lost-reasons", h.lostReasons)
	g.GET("/response-times", h.responseTimes)
	g.GET("/tasks", h.tasks)
}

// parseFilter mirrors internal/lead's created_from/created_to parsing —
// an unparseable or absent value is silently ignored rather than
// rejected, leaving that bound unset (Filter's nil = unbounded).
//
// assigned_to and source (Phase 8.6 TD §2.1) follow the same rule: a
// non-UUID assignee or a source outside the four the database allows is
// treated as not given. assigned_to is the same name the lead list uses.
func parseFilter(c *gin.Context) Filter {
	var f Filter
	if from, err := time.Parse(time.RFC3339, c.Query("from")); err == nil {
		f.From = &from
	}
	if to, err := time.Parse(time.RFC3339, c.Query("to")); err == nil {
		f.To = &to
	}
	if id, err := uuid.Parse(c.Query("assigned_to")); err == nil {
		f.Assignee = &id
	}
	if src := c.Query("source"); slices.Contains(leadSources, src) {
		f.Source = &src
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

func (h *Handler) trend(c *gin.Context) {
	t := authn.TenantFromContext(c)

	tr, err := h.usecase.Trend(c.Request.Context(), t, parseFilter(c))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	points := make([]gin.H, 0, len(tr.Points))
	for _, p := range tr.Points {
		// A calendar date in the organization's timezone, not an instant —
		// so YYYY-MM-DD rather than an ISO timestamp with Z (TD §2.2):
		// printing it as UTC midnight would shift the day for WITA/WIT.
		points = append(points, gin.H{"date": p.Date.Format("2006-01-02"), "count": p.Count})
	}
	httpx.OK(c, http.StatusOK, gin.H{"bucket": tr.Bucket, "points": points})
}

func (h *Handler) sources(c *gin.Context) {
	t := authn.TenantFromContext(c)

	out, err := h.usecase.Sources(c.Request.Context(), t, parseFilter(c))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	data := make([]gin.H, 0, len(out))
	for _, m := range out {
		data = append(data, gin.H{
			"source":          m.Source,
			"count":           m.Count,
			"won_count":       m.WonCount,
			"conversion_rate": m.ConversionRate,
		})
	}
	httpx.OK(c, http.StatusOK, data)
}

func (h *Handler) lostReasons(c *gin.Context) {
	t := authn.TenantFromContext(c)

	out, err := h.usecase.LostReasons(c.Request.Context(), t, parseFilter(c))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	data := make([]gin.H, 0, len(out))
	for _, m := range out {
		data = append(data, gin.H{"reason": m.Reason, "count": m.Count})
	}
	httpx.OK(c, http.StatusOK, data)
}

func (h *Handler) responseTimes(c *gin.Context) {
	t := authn.TenantFromContext(c)

	rt, err := h.usecase.ResponseTimes(c.Request.Context(), t, parseFilter(c))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	buckets := make([]gin.H, 0, len(rt.Buckets))
	for _, b := range rt.Buckets {
		buckets = append(buckets, gin.H{"bucket": b.Bucket, "count": b.Count})
	}
	httpx.OK(c, http.StatusOK, gin.H{"buckets": buckets, "median_seconds": rt.MedianSeconds})
}

func (h *Handler) tasks(c *gin.Context) {
	t := authn.TenantFromContext(c)

	out, err := h.usecase.Tasks(c.Request.Context(), t, parseFilter(c))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	data := make([]gin.H, 0, len(out))
	for _, m := range out {
		data = append(data, gin.H{
			"membership_id":   m.MembershipID,
			"full_name":       m.FullName,
			"completed_count": m.CompletedCount,
			"overdue_count":   m.OverdueCount,
		})
	}
	httpx.OK(c, http.StatusOK, data)
}
