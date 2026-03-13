package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"zui/storage"
)

type InboundHandler struct {
	store *storage.Store
}

type inboundRequest struct {
	Tag            string          `json:"tag"`
	Remark         string          `json:"remark"`
	Protocol       string          `json:"protocol"`
	Listen         string          `json:"listen"`
	Port           int             `json:"port"`
	TotalGB        int64           `json:"totalGB"`
	UsedGB         int64           `json:"usedGB"`
	DeviceLimit    int             `json:"deviceLimit"`
	ExpiryAt       string          `json:"expiryAt"`
	Authentication string          `json:"authentication"`
	Decryption     string          `json:"decryption"`
	Encryption     string          `json:"encryption"`
	Transport      string          `json:"transport"`
	Security       string          `json:"security"`
	ProxyProtocol  *bool           `json:"proxyProtocol"`
	Settings       json.RawMessage `json:"settings"`
	StreamSettings json.RawMessage `json:"streamSettings"`
	Sniffing       json.RawMessage `json:"sniffing"`
	Fallbacks      json.RawMessage `json:"fallbacks"`
	Sockopt        json.RawMessage `json:"sockopt"`
	HTTPObfs       json.RawMessage `json:"httpObfs"`
	ExternalProxy  json.RawMessage `json:"externalProxy"`
	Enable         *bool           `json:"enable"`
}

func NewInboundHandler(store *storage.Store) *InboundHandler {
	return &InboundHandler{store: store}
}

func (h *InboundHandler) List(c *gin.Context) {
	items, err := h.store.ListInbounds()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *InboundHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	item, err := h.store.GetInbound(id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "inbound not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *InboundHandler) Create(c *gin.Context) {
	var req inboundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}

	inbound, err := toStorageInbound(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.store.CreateInbound(inbound)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, h.store, "inbound.create", strconv.FormatInt(item.ID, 10), item.Remark+"/"+item.Protocol)
	c.JSON(http.StatusCreated, item)
}

func (h *InboundHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req inboundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}
	inbound, err := toStorageInbound(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.store.UpdateInbound(id, inbound)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "inbound not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, h.store, "inbound.update", strconv.FormatInt(item.ID, 10), item.Remark+"/"+item.Protocol)
	c.JSON(http.StatusOK, item)
}

func (h *InboundHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := h.store.DeleteInbound(id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "inbound not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, h.store, "inbound.delete", strconv.FormatInt(id, 10), "")
	c.JSON(http.StatusOK, gin.H{"message": "inbound deleted"})
}

func parseIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

func toStorageInbound(req inboundRequest) (storage.Inbound, error) {
	if strings.TrimSpace(req.Remark) == "" {
		return storage.Inbound{}, errors.New("remark is required")
	}
	if !isSupportedProtocol(req.Protocol) {
		return storage.Inbound{}, errors.New("unsupported protocol")
	}
	if req.Port < 1 || req.Port > 65535 {
		return storage.Inbound{}, errors.New("port must be 1-65535")
	}
	if req.DeviceLimit < 0 {
		return storage.Inbound{}, errors.New("deviceLimit must be >= 0")
	}

	enable := true
	if req.Enable != nil {
		enable = *req.Enable
	}
	proxyProtocol := false
	if req.ProxyProtocol != nil {
		proxyProtocol = *req.ProxyProtocol
	}

	listen := strings.TrimSpace(req.Listen)
	if listen == "" {
		listen = "0.0.0.0"
	}

	transport := strings.TrimSpace(req.Transport)
	if transport == "" {
		transport = "tcp"
	}
	security := strings.TrimSpace(req.Security)
	if security == "" {
		security = "none"
	}
	decryption := strings.TrimSpace(req.Decryption)
	if decryption == "" {
		decryption = "none"
	}

	settingsJSON, err := normalizeJSONObject(req.Settings)
	if err != nil {
		return storage.Inbound{}, errors.New("settings must be valid json object")
	}
	streamJSON, err := normalizeJSONObject(req.StreamSettings)
	if err != nil {
		return storage.Inbound{}, errors.New("streamSettings must be valid json object")
	}
	sniffingJSON, err := normalizeJSONObject(req.Sniffing)
	if err != nil {
		return storage.Inbound{}, errors.New("sniffing must be valid json object")
	}
	fallbacksJSON, err := normalizeJSONArray(req.Fallbacks)
	if err != nil {
		return storage.Inbound{}, errors.New("fallbacks must be valid json array")
	}
	sockoptJSON, err := normalizeJSONObject(req.Sockopt)
	if err != nil {
		return storage.Inbound{}, errors.New("sockopt must be valid json object")
	}
	httpObfsJSON, err := normalizeJSONObject(req.HTTPObfs)
	if err != nil {
		return storage.Inbound{}, errors.New("httpObfs must be valid json object")
	}
	externalProxyJSON, err := normalizeJSONObject(req.ExternalProxy)
	if err != nil {
		return storage.Inbound{}, errors.New("externalProxy must be valid json object")
	}

	expiryAt, err := parseOptionalTime(req.ExpiryAt)
	if err != nil {
		return storage.Inbound{}, err
	}

	return storage.Inbound{
		Tag:                strings.TrimSpace(req.Tag),
		Remark:             strings.TrimSpace(req.Remark),
		Protocol:           req.Protocol,
		Listen:             listen,
		Port:               req.Port,
		TotalGB:            req.TotalGB,
		UsedGB:             req.UsedGB,
		DeviceLimit:        req.DeviceLimit,
		ExpiryAt:           expiryAt,
		Authentication:     strings.TrimSpace(req.Authentication),
		Decryption:         decryption,
		Encryption:         strings.TrimSpace(req.Encryption),
		Transport:          transport,
		Security:           security,
		ProxyProtocol:      proxyProtocol,
		SettingsJSON:       settingsJSON,
		StreamSettingsJSON: streamJSON,
		SniffingJSON:       sniffingJSON,
		FallbacksJSON:      fallbacksJSON,
		SockoptJSON:        sockoptJSON,
		HTTPObfsJSON:       httpObfsJSON,
		ExternalProxyJSON:  externalProxyJSON,
		Enable:             enable,
	}, nil
}

func normalizeJSONObject(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "{}", nil
	}
	var temp map[string]any
	if err := json.Unmarshal(raw, &temp); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(temp)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func normalizeJSONArray(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "[]", nil
	}
	var temp []any
	if err := json.Unmarshal(raw, &temp); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(temp)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func parseOptionalTime(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		t := t.UTC()
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
		t := t.UTC()
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		t := t.UTC()
		return &t, nil
	}
	return nil, errors.New("expiryAt must be RFC3339/2006-01-02 15:04:05/2006-01-02")
}

func isSupportedProtocol(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "vmess", "vless", "trojan", "shadowsocks", "shadowsocks-2022", "shadowsocks_2022", "socks", "http":
		return true
	default:
		return false
	}
}
