package lead

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/ratelimit"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// maxPublicLeadBodyBytes is TD phase 4 §5's 64 KB cap on POST /v1/leads
// via API key — enforced BEFORE anything is parsed. Without it,
// raw_payload is a path to writing megabytes to the database in one
// request; the dashboard's own create has no such limit (gin's default
// body handling already applies there).
const maxPublicLeadBodyBytes = 64 * 1024

// publicRateLimiter is declared here (the consumer, ADR-011) —
// *ratelimit.FixedWindow's Take method satisfies it structurally. Only
// what create needs: a key in, a Result out.
type publicRateLimiter interface {
	Take(key string) ratelimit.Result
}

type Handler struct {
	usecase         *Usecase
	publicRateLimit publicRateLimiter
}

func NewHandler(usecase *Usecase, publicRateLimit publicRateLimiter) *Handler {
	return &Handler{usecase: usecase, publicRateLimit: publicRateLimit}
}

// RegisterRoutes mounts /v1/leads. POST is registered on its OWN group
// with publicCreateMW (authn.MiddlewareWithAPIKey) — the only route in
// this package that accepts two principals (Phase 4 TD §3, §9). Every
// other route stays behind authMW alone, which has no notion of an API
// key at all — same two-groups-same-prefix pattern customer's handler
// already uses for POST /v1/leads/{id}/convert.
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW, publicCreateMW gin.HandlerFunc) {
	create := r.Group("/v1/leads")
	create.Use(publicCreateMW)
	create.POST("", h.create)

	g := r.Group("/v1/leads")
	g.Use(authMW)
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.PATCH("/:id/status", h.updateStatus)
	g.PATCH("/:id/assignment", h.updateAssignment)
	g.DELETE("/:id", h.delete)
}

type createRequest struct {
	Name                   string  `json:"name"`
	Email                  *string `json:"email"`
	Phone                  *string `json:"phone"`
	Company                *string `json:"company"`
	Notes                  *string `json:"notes"`
	Source                 string  `json:"source"`
	AssignedToMembershipID *string `json:"assigned_to_membership_id"`
}

// create honors Idempotency-Key (Aturan #34-adjacent convention, TD §7):
// a repeated key returns the ORIGINAL lead with 200, never a second
// lead and never an error.
//
// This is the ONE handler in the whole API two principals reach (TD
// §3, §9): PrincipalUser via authMW (unchanged from Phase 2) and
// PrincipalAPIKey via publicCreateMW (Phase 4). The branches below are
// the only place that distinction is visible — everything past body
// parsing calls the exact same h.usecase.Create either way.
func (h *Handler) create(c *gin.Context) {
	t := authn.TenantFromContext(c)

	if t.PrincipalType == tenant.PrincipalAPIKey {
		// Rate-limit headers are set on EVERY response along this path,
		// success or failure (TD §6: "dikirim sejak versi pertama") —
		// doing it here, before body parsing can fail for any reason,
		// is what makes that unconditional rather than something every
		// later return statement has to remember.
		result := h.publicRateLimit.Take("publiclead:key:" + t.APIKeyID.String())
		httpx.SetRateLimitHeaders(c, result)
		if !result.Allowed {
			httpx.RespondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak request. Coba lagi nanti.")
			return
		}
	}

	var req createRequest
	var rawPayload []byte

	if t.PrincipalType == tenant.PrincipalAPIKey {
		// raw_payload needs the body's exact bytes (TD §5) — ShouldBindJSON
		// only decodes, it never hands the raw bytes back. MaxBytesReader
		// enforces the 64 KB cap before a single byte is parsed.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPublicLeadBodyBytes)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				httpx.WriteError(c, payloadTooLargeError())
				return
			}
			httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
			return
		}
		rawPayload = body
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
				return
			}
		}
	} else if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	in := CreateLeadInput{
		Name: req.Name, Email: req.Email, Phone: req.Phone, Company: req.Company, Notes: req.Notes,
		Source: req.Source, RawPayload: rawPayload,
	}
	if assignee, err := parseUUIDPtr(req.AssignedToMembershipID); err == nil {
		in.AssignedToMembershipID = assignee
	}
	if key := c.GetHeader("Idempotency-Key"); key != "" {
		in.IdempotencyKey = &key
	}

	created, isNew, err := h.usecase.Create(c.Request.Context(), t, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	status := http.StatusOK
	if isNew {
		status = http.StatusCreated
	}
	httpx.OK(c, status, leadJSON(created))
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
	httpx.OK(c, http.StatusOK, leadJSON(found))
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	in := ListInput{
		Status: splitCSV(c.Query("status")),
		Source: splitCSV(c.Query("source")),
		Query:  c.Query("q"),
	}
	if assignedTo := c.Query("assigned_to"); assignedTo == "none" {
		in.AssignedToNone = true
	} else if assignedTo != "" {
		if id, err := uuid.Parse(assignedTo); err == nil {
			in.AssignedTo = &id
		}
	}
	if from, err := time.Parse(time.RFC3339, c.Query("created_from")); err == nil {
		in.CreatedFrom = &from
	}
	if to, err := time.Parse(time.RFC3339, c.Query("created_to")); err == nil {
		in.CreatedTo = &to
	}
	if page, err := strconv.Atoi(c.Query("page")); err == nil {
		in.Page = page
	}
	if perPage, err := strconv.Atoi(c.Query("per_page")); err == nil {
		in.PerPage = perPage
	}

	leads, meta, err := h.usecase.List(c.Request.Context(), t, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(leads))
	for _, l := range leads {
		out = append(out, leadJSON(l))
	}
	httpx.List(c, out, meta)
}

type updateRequest struct {
	Version int     `json:"version"`
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

	updated, err := h.usecase.Update(c.Request.Context(), t, id, UpdateLeadInput(req))
	if err != nil {
		respondLeadError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, leadJSON(updated))
}

type updateStatusRequest struct {
	Version    int     `json:"version"`
	Status     string  `json:"status"`
	LostReason *string `json:"lost_reason"`
}

func (h *Handler) updateStatus(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	updated, err := h.usecase.UpdateStatus(c.Request.Context(), t, id, UpdateStatusInput(req))
	if err != nil {
		respondLeadError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, leadJSON(updated))
}

type updateAssignmentRequest struct {
	Version                int     `json:"version"`
	AssignedToMembershipID *string `json:"assigned_to_membership_id"`
}

func (h *Handler) updateAssignment(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	var req updateAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	in := UpdateAssignmentInput{Version: req.Version}
	if assignee, err := parseUUIDPtr(req.AssignedToMembershipID); err == nil {
		in.AssignedToMembershipID = assignee
	}

	updated, err := h.usecase.UpdateAssignment(c.Request.Context(), t, id, in)
	if err != nil {
		respondLeadError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, leadJSON(updated))
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

// respondLeadError type-switches for *VersionConflictError before
// falling through to the generic mapper — TD §4's 409 body carries the
// current row, which httpx.DomainError can't express (same pattern as
// auth's organization_selection_required).
func respondLeadError(c *gin.Context, err error) {
	var conflict *VersionConflictError
	if errors.As(err, &conflict) {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{
			"code":    "version_conflict",
			"message": "Data sudah diubah oleh orang lain. Muat ulang dan coba lagi.",
			"current": leadJSON(conflict.Current),
		}})
		return
	}
	httpx.WriteError(c, err)
}

func leadJSON(l *Lead) gin.H {
	return gin.H{
		"id":                        l.ID,
		"lead_number":               l.LeadNumber,
		"name":                      l.Name,
		"email":                     l.Email,
		"phone":                     l.Phone,
		"phone_e164":                l.PhoneE164,
		"company":                   l.Company,
		"notes":                     l.Notes,
		"status":                    l.Status,
		"lost_reason":               l.LostReason,
		"source":                    l.Source,
		"source_api_key_id":         l.SourceAPIKeyID,
		"assigned_to_membership_id": l.AssignedToMembershipID,
		"version":                   l.Version,
		"created_by_membership_id":  l.CreatedByMembershipID,
		"created_at":                l.CreatedAt,
		"updated_at":                l.UpdatedAt,
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func parseUUIDPtr(s *string) (*uuid.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func payloadTooLargeError() error {
	return &httpx.DomainError{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    "payload_too_large",
		Message: "Isi request melebihi batas 64 KB.",
	}
}
