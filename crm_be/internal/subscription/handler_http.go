package subscription

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

// RegisterRoutes mounts GET /v1/plans — #125's plan comparison screen,
// the first HTTP endpoint this package owns directly (every other
// entry point until now was called from another domain's usecase via a
// PlanGate bridge, per ADR-011).
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/plans")
	g.Use(authMW)
	g.GET("", h.list)
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	plans, err := h.usecase.ListPlans(c.Request.Context(), t)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(plans))
	for _, p := range plans {
		out = append(out, gin.H{
			"code":        p.Code,
			"name":        p.Name,
			"price_label": p.PriceLabel,
			"limits": gin.H{
				"leads_per_month": p.Limits.LeadsPerMonth,
				"seats":           p.Limits.Seats,
			},
			"channels": stringKeyed(p.Channels),
		})
	}

	httpx.OK(c, http.StatusOK, out)
}
