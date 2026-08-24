package task

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
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

// RegisterRoutes mounts two groups: a lead-scoped one sharing
// /v1/leads' :id wildcard (same discipline as activity.Handler — gin
// requires the same wildcard NAME wherever paths share a prefix), and
// task's own /v1/tasks resource group.
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	leads := r.Group("/v1/leads")
	leads.Use(authMW)
	leads.POST("/:id/tasks", h.create)
	leads.GET("/:id/tasks", h.listByLead)

	tasks := r.Group("/v1/tasks")
	tasks.Use(authMW)
	tasks.GET("", h.listByOrg)
	tasks.PATCH("/:id", h.update)
	tasks.POST("/:id/complete", h.complete)
	tasks.DELETE("/:id", h.delete)
}

type createRequest struct {
	Title                  string     `json:"title"`
	Description            *string    `json:"description"`
	DueAt                  *time.Time `json:"due_at"`
	AssignedToMembershipID *string    `json:"assigned_to_membership_id"`
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

	in := CreateTaskInput{Title: req.Title, Description: req.Description, DueAt: req.DueAt}
	if assignee, err := parseUUIDPtr(req.AssignedToMembershipID); err == nil {
		in.AssignedToMembershipID = assignee
	}

	created, err := h.usecase.Create(c.Request.Context(), t, leadID, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, taskJSON(created))
}

func (h *Handler) listByLead(c *gin.Context) {
	t := authn.TenantFromContext(c)

	leadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	tasks, err := h.usecase.ListByLead(c.Request.Context(), t, leadID)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(tasks))
	for _, tsk := range tasks {
		out = append(out, taskJSON(tsk))
	}
	httpx.OK(c, http.StatusOK, out)
}

func (h *Handler) listByOrg(c *gin.Context) {
	t := authn.TenantFromContext(c)

	in := ListTasksInput{Status: splitCSV(c.Query("status"))}
	if assignedTo := c.Query("assigned_to"); assignedTo != "" {
		if id, err := uuid.Parse(assignedTo); err == nil {
			in.AssignedTo = &id
		}
	}
	if due, err := time.Parse(time.RFC3339, c.Query("due_before")); err == nil {
		in.DueBefore = &due
	}
	if page, err := strconv.Atoi(c.Query("page")); err == nil {
		in.Page = page
	}
	if perPage, err := strconv.Atoi(c.Query("per_page")); err == nil {
		in.PerPage = perPage
	}

	tasks, meta, err := h.usecase.ListByOrg(c.Request.Context(), t, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(tasks))
	for _, tsk := range tasks {
		out = append(out, taskJSON(tsk))
	}
	httpx.List(c, out, meta)
}

type updateRequest struct {
	Version                int        `json:"version"`
	Title                  *string    `json:"title"`
	Description            *string    `json:"description"`
	DueAt                  *time.Time `json:"due_at"`
	AssignedToMembershipID *string    `json:"assigned_to_membership_id"`
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

	in := UpdateTaskInput{Version: req.Version, Title: req.Title, Description: req.Description, DueAt: req.DueAt}
	if assignee, err := parseUUIDPtr(req.AssignedToMembershipID); err == nil {
		in.AssignedToMembershipID = assignee
	}

	updated, err := h.usecase.Update(c.Request.Context(), t, id, in)
	if err != nil {
		respondTaskError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, taskJSON(updated))
}

type completeRequest struct {
	Version int `json:"version"`
}

func (h *Handler) complete(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	completed, err := h.usecase.Complete(c.Request.Context(), t, id, req.Version)
	if err != nil {
		respondTaskError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, taskJSON(completed))
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

// respondTaskError type-switches for *VersionConflictError before
// falling through to the generic mapper — same pattern as lead's
// respondLeadError.
func respondTaskError(c *gin.Context, err error) {
	var conflict *VersionConflictError
	if errors.As(err, &conflict) {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{
			"code":    "version_conflict",
			"message": "Data sudah diubah oleh orang lain. Muat ulang dan coba lagi.",
			"current": taskJSON(conflict.Current),
		}})
		return
	}
	httpx.WriteError(c, err)
}

func taskJSON(tsk *Task) gin.H {
	return gin.H{
		"id":                         tsk.ID,
		"lead_id":                    tsk.LeadID,
		"title":                      tsk.Title,
		"description":                tsk.Description,
		"due_at":                     tsk.DueAt,
		"status":                     tsk.Status,
		"assigned_to_membership_id":  tsk.AssignedToMembershipID,
		"completed_at":               tsk.CompletedAt,
		"completed_by_membership_id": tsk.CompletedByMembershipID,
		"version":                    tsk.Version,
		"created_by_membership_id":   tsk.CreatedByMembershipID,
		"created_at":                 tsk.CreatedAt,
		"updated_at":                 tsk.UpdatedAt,
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
