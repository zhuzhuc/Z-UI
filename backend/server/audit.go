package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"zui/storage"
)

type AuditHandler struct {
	store *storage.Store
}

func NewAuditHandler(store *storage.Store) *AuditHandler {
	return &AuditHandler{store: store}
}

func (h *AuditHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "100")))
	offset, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("offset", "0")))
	items, err := h.store.ListAuditLogs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "limit": limit, "offset": offset})
}

func recordAudit(c *gin.Context, store *storage.Store, action, target, detail string) {
	if store == nil {
		return
	}
	username, _ := c.Get("auth.username")
	_ = store.AddAuditLog(action, target, detail, toString(username), c.ClientIP())
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

