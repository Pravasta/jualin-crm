package subscription

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

// HandlerConfig carries deployment facts GET /v1/plans reports that are
// neither plan policy nor tenant data. It lives on the handler rather
// than on Usecase for the same reason auth.MeConfig does: it is
// configuration, and a usecase must not learn to read config (ADR-011).
//
// Keeping it out of planDisplay matters for a second reason too — the
// catalog in plan.go is a pure function of product policy, identical in
// every deployment. A contact URL is not: it differs per deployment and
// changes without the plans changing.
type HandlerConfig struct {
	// EnterpriseContactURL is where the Enterprise card sends someone
	// who wants to negotiate (prd 8.5 D4). Empty means the dashboard
	// renders the card WITHOUT a link rather than with a dead one.
	EnterpriseContactURL string
}

type Handler struct {
	usecase *Usecase
	cfg     HandlerConfig
}

func NewHandler(usecase *Usecase, cfg HandlerConfig) *Handler {
	return &Handler{usecase: usecase, cfg: cfg}
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
		entry := gin.H{
			"code":        p.Code,
			"name":        p.Name,
			"price_label": p.PriceLabel,
			"limits": gin.H{
				"leads_per_month": p.Limits.LeadsPerMonth,
				"seats":           p.Limits.Seats,
			},
			"channels": stringKeyed(p.Channels),
		}
		// Only Enterprise has a contact destination, and only when the
		// deployment configured one. The field is OMITTED rather than
		// sent empty so the dashboard's check is "is there a link" — not
		// "is this string non-empty", which is the kind of condition that
		// eventually renders href="".
		if p.Code == PlanEnterprise && h.cfg.EnterpriseContactURL != "" {
			entry["contact_url"] = h.cfg.EnterpriseContactURL
		}
		out = append(out, entry)
	}

	httpx.OK(c, http.StatusOK, out)
}
