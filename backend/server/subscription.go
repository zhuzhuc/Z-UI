package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
	"zui/storage"
)

type SubscriptionHandler struct {
	store *storage.Store
}

func NewSubscriptionHandler(store *storage.Store) *SubscriptionHandler {
	return &SubscriptionHandler{store: store}
}

func (h *SubscriptionHandler) Info(c *gin.Context) {
	settings, err := h.ensureToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	baseURL := h.resolveBaseURL(c, settings.PublicBaseURL)
	subURL := strings.TrimSuffix(baseURL, "/") + "/api/v1/sub/" + settings.SubscriptionToken
	qrDataURL, err := buildQRCodeDataURL(subURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	links, err := h.buildLinks(c.Request.Host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         settings.SubscriptionToken,
		"url":           subURL,
		"qrDataUrl":     qrDataURL,
		"linkCount":     len(links),
		"publicBaseUrl": settings.PublicBaseURL,
	})
}

func (h *SubscriptionHandler) Rotate(c *gin.Context) {
	settings, err := h.store.GetPanelSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	settings.SubscriptionToken = randomToken(24)
	updated, err := h.store.UpdatePanelSettings(settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, h.store, "subscription.rotate_token", "subscription", "")

	baseURL := h.resolveBaseURL(c, updated.PublicBaseURL)
	c.JSON(http.StatusOK, gin.H{
		"message": "subscription token rotated",
		"token":   updated.SubscriptionToken,
		"url":     strings.TrimSuffix(baseURL, "/") + "/api/v1/sub/" + updated.SubscriptionToken,
	})
}

func (h *SubscriptionHandler) Preview(c *gin.Context) {
	links, err := h.buildLinks(c.Request.Host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	raw := strings.Join(links, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	c.JSON(http.StatusOK, gin.H{
		"raw":    raw,
		"base64": encoded,
		"count":  len(links),
		"items":  links,
	})
}

func (h *SubscriptionHandler) Nodes(c *gin.Context) {
	host := hostWithoutPort(c.Request.Host)
	userNetMap := loadRecentUserNetworkMap()
	inbounds, err := h.store.ListInbounds()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]gin.H, 0, len(inbounds))
	totalLinks := 0
	for _, inb := range inbounds {
		links, err := inboundToLinks(inb, host)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		users, err := extractNodeUsers(inb.Protocol, inb.SettingsJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		nodeItems := make([]gin.H, 0, len(links))
		for i, link := range links {
			qrDataURL, err := buildQRCodeDataURL(link)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			user := ""
			if i < len(users) {
				user = strings.TrimSpace(users[i])
			}
			netInfo := userNetMap[normalizeUserKey(user)]
			nodeItems = append(nodeItems, gin.H{
				"name":      nodeDisplayName(link, i),
				"user":      user,
				"statsKey":  user,
				"clientIP":  netInfo.IP,
				"lastActiveIP": netInfo.IP,
				"clientMac": netInfo.MAC,
				"lastSeen":  netInfo.LastSeen,
				"url":       link,
				"qrDataUrl": qrDataURL,
			})
		}
		totalLinks += len(nodeItems)
		items = append(items, gin.H{
			"inboundId": inb.ID,
			"remark":    inb.Remark,
			"protocol":  inb.Protocol,
			"port":      inb.Port,
			"enable":    inb.Enable,
			"nodes":     nodeItems,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"count":     len(items),
		"linkCount": totalLinks,
		"items":     items,
	})
}

type userNetworkInfo struct {
	IP  string
	MAC string
	LastSeen string
}

var (
	reIPv4       = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	reMAC        = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}\b`)
	reEmailJSON  = regexp.MustCompile(`"email"\s*:\s*"([^"]+)"`)
	reEmailPlain = regexp.MustCompile(`(?i)\bemail\s*[:=]\s*"?([^\s",]+)"?`)
	reUserJSON   = regexp.MustCompile(`"user"\s*:\s*"([^"]+)"`)
	reUserPlain  = regexp.MustCompile(`(?i)\buser\s*[:=]\s*"?([^\s",]+)"?`)
	reTSRFC3339  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
	reTSYMDHMS   = regexp.MustCompile(`\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2}`)
	reTSYMDHMS2  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`)
)

func loadRecentUserNetworkMap() map[string]userNetworkInfo {
	content := ""
	if path := resolveXrayAccessLogPath(); path != "" {
		if one, err := tailFileLines(path, 4000); err == nil {
			content = one
		}
	}
	out := map[string]userNetworkInfo{}
	if strings.TrimSpace(content) == "" {
		return out
	}

	for _, line := range strings.Split(content, "\n") {
		user := extractUserFromLogLine(line)
		if user == "" {
			continue
		}
		key := normalizeUserKey(user)
		if key == "" {
			continue
		}
		item := out[key]
		if ip := extractIPv4FromLogLine(line); ip != "" {
			item.IP = ip
		}
		if mac := extractMACFromLogLine(line); mac != "" {
			item.MAC = mac
		}
		if ts, ok := extractTimestampFromLogLine(line); ok {
			item.LastSeen = ts.Format(time.RFC3339)
		}
		out[key] = item
	}
	return out
}

func resolveXrayAccessLogPath() string {
	candidates := []string{}
	if v := strings.TrimSpace(os.Getenv("XRAY_ACCESS_LOG")); v != "" {
		candidates = append(candidates, v)
	}
	if cfgPath := strings.TrimSpace(os.Getenv("XRAY_CONFIG")); cfgPath != "" {
		if logCfg, err := parseXrayLogConfig(cfgPath); err == nil && strings.TrimSpace(logCfg.Access) != "" {
			candidates = append(candidates, strings.TrimSpace(logCfg.Access))
		}
	}
	cfgPath := strings.TrimSpace(os.Getenv("XRAY_CONFIG"))
	if cfgPath == "" {
		cfgPath = "./runtime/xray-config.json"
	}
	candidates = append(candidates,
		filepath.Join(filepath.Dir(cfgPath), "xray-access.log"),
		"./runtime/xray-access.log",
		"../runtime/xray-access.log",
	)
	for _, one := range candidates {
		if one == "" {
			continue
		}
		if stat, err := os.Stat(one); err == nil && !stat.IsDir() {
			return one
		}
	}
	return ""
}

func normalizeUserKey(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func extractUserFromLogLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if m := reEmailJSON.FindStringSubmatch(line); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := reEmailPlain.FindStringSubmatch(line); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := reUserJSON.FindStringSubmatch(line); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := reUserPlain.FindStringSubmatch(line); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractIPv4FromLogLine(line string) string {
	if m := reIPv4.FindString(line); m != "" {
		return m
	}
	return ""
}

func extractMACFromLogLine(line string) string {
	if m := reMAC.FindString(line); m != "" {
		return strings.ToLower(m)
	}
	return ""
}

func extractTimestampFromLogLine(line string) (time.Time, bool) {
	if m := reTSRFC3339.FindString(line); m != "" {
		if t, err := time.Parse(time.RFC3339, m); err == nil {
			return t.UTC(), true
		}
	}
	if m := reTSYMDHMS.FindString(line); m != "" {
		if t, err := time.Parse("2006/01/02 15:04:05", m); err == nil {
			return t.UTC(), true
		}
	}
	if m := reTSYMDHMS2.FindString(line); m != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", m); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func (h *SubscriptionHandler) PublicSubscription(c *gin.Context) {
	settings, err := h.ensureToken()
	if err != nil {
		c.String(http.StatusInternalServerError, "internal error")
		return
	}
	if c.Param("token") != settings.SubscriptionToken {
		c.String(http.StatusUnauthorized, "invalid token")
		return
	}

	links, err := h.buildLinks(c.Request.Host)
	if err != nil {
		c.String(http.StatusInternalServerError, "generate subscription failed")
		return
	}
	raw := strings.Join(links, "\n")
	if c.Query("raw") == "1" {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, raw)
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, base64.StdEncoding.EncodeToString([]byte(raw)))
}

func (h *SubscriptionHandler) ensureToken() (storage.PanelSettings, error) {
	settings, err := h.store.GetPanelSettings()
	if err != nil {
		return storage.PanelSettings{}, err
	}
	if settings.SubscriptionToken != "" {
		return settings, nil
	}
	settings.SubscriptionToken = randomToken(24)
	return h.store.UpdatePanelSettings(settings)
}

func (h *SubscriptionHandler) resolveBaseURL(c *gin.Context, configured string) string {
	if configured != "" {
		return configured
	}
	if env := os.Getenv("PANEL_PUBLIC_BASE"); env != "" {
		return env
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

func (h *SubscriptionHandler) buildLinks(host string) ([]string, error) {
	host = hostWithoutPort(host)
	inbounds, err := h.store.ListEnabledInbounds()
	if err != nil {
		return nil, err
	}

	links := make([]string, 0)
	for _, inb := range inbounds {
		oneLinks, err := inboundToLinks(inb, host)
		if err != nil {
			return nil, err
		}
		links = append(links, oneLinks...)
	}
	return links, nil
}

func inboundToLinks(inb storage.Inbound, host string) ([]string, error) {
	streamSettings := map[string]any{}
	_ = json.Unmarshal([]byte(inb.StreamSettingsJSON), &streamSettings)
	network := stringFromMap(streamSettings, "network", "tcp")
	security := stringFromMap(streamSettings, "security", "none")
	tagPrefix := sanitizeRemark(inb.Remark)
	portText := strconv.Itoa(inb.Port)
	if inb.Protocol == "shadowsocks" || inb.Protocol == "shadowsocks-2022" || inb.Protocol == "shadowsocks_2022" {
		return buildShadowsocksLinks(inb, host, portText, tagPrefix)
	}

	clients, err := extractClients(inb.Protocol, inb.SettingsJSON)
	if err != nil {
		return nil, err
	}
	links := make([]string, 0, len(clients))
	for i, client := range clients {
		label := fmt.Sprintf("%s-%d", tagPrefix, i+1)
		if client.Email != "" {
			label = client.Email
		}
		switch inb.Protocol {
		case "vless":
			query := url.Values{}
			query.Set("encryption", "none")
			query.Set("security", security)
			query.Set("type", network)
			links = append(links, fmt.Sprintf(
				"vless://%s@%s:%s?%s#%s",
				client.IDOrPassword,
				host,
				portText,
				query.Encode(),
				url.QueryEscape(label),
			))
		case "trojan":
			query := url.Values{}
			query.Set("security", security)
			query.Set("type", network)
			links = append(links, fmt.Sprintf(
				"trojan://%s@%s:%s?%s#%s",
				client.IDOrPassword,
				host,
				portText,
				query.Encode(),
				url.QueryEscape(label),
			))
		case "vmess":
			vmessObj := map[string]string{
				"v":    "2",
				"ps":   label,
				"add":  host,
				"port": portText,
				"id":   client.IDOrPassword,
				"aid":  "0",
				"net":  network,
				"type": "none",
				"host": "",
				"path": "",
				"tls":  mapSecurityToTLS(security),
			}
			raw, _ := json.Marshal(vmessObj)
			links = append(links, "vmess://"+base64.StdEncoding.EncodeToString(raw))
		}
	}
	return links, nil
}

func buildShadowsocksLinks(inb storage.Inbound, host, portText, fallbackLabel string) ([]string, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(inb.SettingsJSON), &settings); err != nil {
		return nil, fmt.Errorf("invalid shadowsocks settings: %w", err)
	}

	method := stringFromAny(settings["method"])
	if method == "" {
		method = "aes-128-gcm"
	}

	accounts := make([]parsedClient, 0)
	password := stringFromAny(settings["password"])
	if password != "" {
		accounts = append(accounts, parsedClient{IDOrPassword: password, Email: stringFromAny(settings["email"])})
	}
	if users, ok := settings["users"].([]any); ok {
		for _, raw := range users {
			user, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			pwd := stringFromAny(user["password"])
			if pwd == "" {
				continue
			}
			accounts = append(accounts, parsedClient{
				IDOrPassword: pwd,
				Email:        stringFromAny(user["email"]),
			})
		}
	}
	if len(accounts) == 0 {
		return []string{}, nil
	}

	out := make([]string, 0, len(accounts))
	for idx, account := range accounts {
		label := fallbackLabel
		if account.Email != "" {
			label = account.Email
		} else if len(accounts) > 1 {
			label = fmt.Sprintf("%s-%d", fallbackLabel, idx+1)
		}
		userInfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + account.IDOrPassword))
		out = append(out, fmt.Sprintf("ss://%s@%s:%s#%s", userInfo, host, portText, url.QueryEscape(label)))
	}
	return out, nil
}

type parsedClient struct {
	IDOrPassword string
	Email        string
}

func extractClients(protocol, settingsJSON string) ([]parsedClient, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return nil, fmt.Errorf("invalid settings json: %w", err)
	}
	rawClients, ok := settings["clients"].([]any)
	if !ok || len(rawClients) == 0 {
		return []parsedClient{}, nil
	}

	clients := make([]parsedClient, 0, len(rawClients))
	for _, raw := range rawClients {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := ""
		switch protocol {
		case "vless", "vmess":
			id = stringFromAny(item["id"])
		case "trojan":
			id = stringFromAny(item["password"])
		}
		if id == "" {
			continue
		}
		clients = append(clients, parsedClient{
			IDOrPassword: id,
			Email:        stringFromAny(item["email"]),
		})
	}
	return clients, nil
}

func stringFromMap(m map[string]any, key, fallback string) string {
	v, ok := m[key]
	if !ok {
		return fallback
	}
	s := stringFromAny(v)
	if s == "" {
		return fallback
	}
	return s
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func mapSecurityToTLS(v string) string {
	if strings.EqualFold(v, "tls") {
		return "tls"
	}
	return ""
}

func sanitizeRemark(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "zui"
	}
	return v
}

func hostWithoutPort(host string) string {
	if h, p, err := net.SplitHostPort(host); err == nil {
		if p != "" {
			return h
		}
	}
	if i := strings.Index(host, ":"); i > -1 && strings.Count(host, ":") == 1 {
		return host[:i]
	}
	return host
}

func randomToken(n int) string {
	if n <= 0 {
		n = 24
	}
	raw := make([]byte, n)
	_, _ = rand.Read(raw)
	return base64.RawURLEncoding.EncodeToString(raw)[:n]
}

func buildQRCodeDataURL(text string) (string, error) {
	png, err := qrcode.Encode(text, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

func extractNodeUsers(protocol, settingsJSON string) ([]string, error) {
	if protocol == "shadowsocks" || protocol == "shadowsocks-2022" || protocol == "shadowsocks_2022" {
		var settings map[string]any
		if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
			return nil, fmt.Errorf("invalid shadowsocks settings: %w", err)
		}
		users := make([]string, 0)
		if rawUsers, ok := settings["users"].([]any); ok {
			for _, raw := range rawUsers {
				one, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				users = append(users, strings.TrimSpace(stringFromAny(one["email"])))
			}
		}
		if len(users) == 0 {
			users = append(users, strings.TrimSpace(stringFromAny(settings["email"])))
		}
		return users, nil
	}

	clients, err := extractClients(protocol, settingsJSON)
	if err != nil {
		return nil, err
	}
	users := make([]string, 0, len(clients))
	for _, client := range clients {
		users = append(users, strings.TrimSpace(client.Email))
	}
	return users, nil
}

func nodeDisplayName(link string, idx int) string {
	fallback := fmt.Sprintf("node-%d", idx+1)
	if strings.HasPrefix(link, "vmess://") {
		raw := strings.TrimPrefix(link, "vmess://")
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(raw)
		}
		if err == nil {
			var vmess map[string]any
			if json.Unmarshal(decoded, &vmess) == nil {
				if ps, ok := vmess["ps"].(string); ok && strings.TrimSpace(ps) != "" {
					return ps
				}
			}
		}
		return fallback
	}
	sharp := strings.LastIndex(link, "#")
	if sharp > -1 && sharp+1 < len(link) {
		if decoded, err := url.QueryUnescape(link[sharp+1:]); err == nil && strings.TrimSpace(decoded) != "" {
			return decoded
		}
	}
	return fallback
}
