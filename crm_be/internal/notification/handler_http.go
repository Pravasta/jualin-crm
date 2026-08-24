package notification

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

func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/notifications")
	g.Use(authMW)
	g.GET("", h.list)
	g.POST("/:id/read", h.markRead)
	g.POST("/read-all", h.markAllRead)
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)
	unreadOnly := c.Query("unread") == "true"

	notifications, err := h.usecase.List(c.Request.Context(), t, unreadOnly)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(notifications))
	for _, n := range notifications {
		out = append(out, notificationJSON(n))
	}
	httpx.OK(c, http.StatusOK, out)
}

func (h *Handler) markRead(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	if err := h.usecase.MarkRead(c.Request.Context(), t, id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) markAllRead(c *gin.Context) {
	t := authn.TenantFromContext(c)

	if err := h.usecase.MarkAllRead(c.Request.Context(), t); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func notificationJSON(n *Notification) gin.H {
	return gin.H{
		"id":         n.ID,
		"type":       n.Type,
		"lead_id":    n.LeadID,
		"task_id":    n.TaskID,
		"title":      n.Title,
		"body":       n.Body,
		"read_at":    n.ReadAt,
		"created_at": n.CreatedAt,
	}
}
