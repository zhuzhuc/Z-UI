package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"zui/storage"
)

type DashboardHandler struct {
	store     *storage.Store
	xray      *XrayManager
	startTime time.Time
}

func NewDashboardHandler(store *storage.Store, xray *XrayManager, startTime time.Time) *DashboardHandler {
	return &DashboardHandler{
		store:     store,
		xray:      xray,
		startTime: startTime,
	}
}

func (h *DashboardHandler) Summary(c *gin.Context) {
	total, enabled, err := h.store.CountInbounds()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	totalGB, usedGB, err := h.store.SumInboundTrafficGB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	xrayStatus, err := h.xray.Snapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	system := collectSystemMetrics()

	c.JSON(http.StatusOK, gin.H{
		"inboundTotal":   total,
		"inboundEnabled": enabled,
		"trafficTotalGB": totalGB,
		"trafficUsedGB":  usedGB,
		"xray":           xrayStatus,
		"system":         system,
		"uptimeSec":      int(time.Since(h.startTime).Seconds()),
		"serverTime":     time.Now(),
	})
}
