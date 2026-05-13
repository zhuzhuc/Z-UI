package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"zui/storage"
)

type SettingsHandler struct {
	store *storage.Store
}

type panelSettingsResponse struct {
	Title                string `json:"title"`
	Language             string `json:"language"`
	Theme                string `json:"theme"`
	RefreshIntervalSec   int    `json:"refreshIntervalSec"`
	RequireLogin         bool   `json:"requireLogin"`
	AllowRegister        bool   `json:"allowRegister"`
	EnableTwoFactorLogin bool   `json:"enableTwoFactorLogin"`
	PublicBaseURL        string `json:"publicBaseUrl"`
	AdminUsername        string `json:"adminUsername"`
}

type publicPanelSettingsResponse struct {
	Title              string `json:"title"`
	Language           string `json:"language"`
	Theme              string `json:"theme"`
	RefreshIntervalSec int    `json:"refreshIntervalSec"`
	RequireLogin       bool   `json:"requireLogin"`
	AllowRegister      bool   `json:"allowRegister"`
	PublicBaseURL      string `json:"publicBaseUrl"`
}

type updatePanelSettingsRequest struct {
	Title                string `json:"title"`
	Language             string `json:"language"`
	Theme                string `json:"theme"`
	RefreshIntervalSec   int    `json:"refreshIntervalSec"`
	RequireLogin         bool   `json:"requireLogin"`
	AllowRegister        bool   `json:"allowRegister"`
	EnableTwoFactorLogin bool   `json:"enableTwoFactorLogin"`
	PublicBaseURL        string `json:"publicBaseUrl"`
}

func NewSettingsHandler(store *storage.Store) *SettingsHandler {
	return &SettingsHandler{store: store}
}

func (h *SettingsHandler) Public(c *gin.Context) {
	item, err := h.store.GetPanelSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toPublicPanelSettingsResponse(item))
}

func (h *SettingsHandler) Get(c *gin.Context) {
	item, err := h.store.GetPanelSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	role := getContextRole(c)
	c.JSON(http.StatusOK, toPanelSettingsResponse(item, role))
}

func (h *SettingsHandler) Update(c *gin.Context) {
	var req updatePanelSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}

	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	if req.Language == "" {
		req.Language = "zh-CN"
	}
	if req.Theme == "" {
		req.Theme = "default"
	}
	if req.RefreshIntervalSec <= 0 {
		req.RefreshIntervalSec = 30
	}
	current, err := h.store.GetPanelSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	publicBaseURL := current.PublicBaseURL
	if req.PublicBaseURL != "" {
		publicBaseURL = req.PublicBaseURL
	}

	item, err := h.store.UpdatePanelSettings(storage.PanelSettings{
		Title:                req.Title,
		Language:             req.Language,
		Theme:                req.Theme,
		RefreshIntervalSec:   req.RefreshIntervalSec,
		RequireLogin:         req.RequireLogin,
		AllowRegister:        req.AllowRegister,
		EnableTwoFactorLogin: req.EnableTwoFactorLogin,
		SubscriptionToken:    current.SubscriptionToken,
		PublicBaseURL:        publicBaseURL,
		AdminUsername:        current.AdminUsername,
		AdminPasswordHash:    current.AdminPasswordHash,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, h.store, "settings.update", "panel", item.Title)

	role := getContextRole(c)
	c.JSON(http.StatusOK, toPanelSettingsResponse(item, role))
}

func toPanelSettingsResponse(item storage.PanelSettings, role string) panelSettingsResponse {
	resp := panelSettingsResponse{
		Title:                item.Title,
		Language:             item.Language,
		Theme:                item.Theme,
		RefreshIntervalSec:   item.RefreshIntervalSec,
		RequireLogin:         item.RequireLogin,
		AllowRegister:        item.AllowRegister,
		EnableTwoFactorLogin: item.EnableTwoFactorLogin,
		PublicBaseURL:        item.PublicBaseURL,
	}
	// Only expose admin username to admin/owner roles
	if hasAdminPrivilege(role) {
		resp.AdminUsername = item.AdminUsername
	}
	return resp
}

func toPublicPanelSettingsResponse(item storage.PanelSettings) publicPanelSettingsResponse {
	return publicPanelSettingsResponse{
		Title:              item.Title,
		Language:           item.Language,
		Theme:              item.Theme,
		RefreshIntervalSec: item.RefreshIntervalSec,
		RequireLogin:       item.RequireLogin,
		AllowRegister:      item.AllowRegister,
		PublicBaseURL:      item.PublicBaseURL,
	}
}
