package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"zui/storage"
)

type XrayManager struct {
	mu          sync.Mutex
	store       *storage.Store
	cmd         *exec.Cmd
	xrayPath    string
	configPath  string
	controlMode string
	serviceName string
	apiAddr     string
}

type xrayLogPaths struct {
	Access string
	Error  string
}

type XrayStatus struct {
	Running     bool   `json:"running"`
	PID         int    `json:"pid"`
	XrayPath    string `json:"xrayPath"`
	ConfigPath  string `json:"configPath"`
	ControlMode string `json:"controlMode"`
	ServiceName string `json:"serviceName"`
	APIAddr     string `json:"apiAddr"`
}

type updateConfigRequest struct {
	XrayPath   string `json:"xrayPath"`
	ConfigPath string `json:"configPath"`
	APIAddr    string `json:"apiAddr"`
}

type applyConfigRequest struct {
	Restart *bool `json:"restart"`
}

func NewXrayManager(store *storage.Store) *XrayManager {
	xrayPath := os.Getenv("XRAY_BIN")
	if xrayPath == "" {
		xrayPath = detectDefaultXrayBin()
	}

	configPath := os.Getenv("XRAY_CONFIG")
	if configPath == "" {
		configPath = "./runtime/xray-config.json"
	}

	controlMode := strings.ToLower(os.Getenv("XRAY_CONTROL"))
	if controlMode == "" {
		controlMode = "process"
	}
	if controlMode != "systemd" {
		controlMode = "process"
	}

	serviceName := os.Getenv("XRAY_SERVICE")
	if serviceName == "" {
		serviceName = "xray"
	}

	apiAddr := os.Getenv("XRAY_API_ADDR")
	if apiAddr == "" {
		apiAddr = "127.0.0.1:10085"
	}

	return &XrayManager{
		store:       store,
		xrayPath:    xrayPath,
		configPath:  configPath,
		controlMode: controlMode,
		serviceName: serviceName,
		apiAddr:     apiAddr,
	}
}

func (m *XrayManager) defaultLogPaths() xrayLogPaths {
	baseDir := filepath.Dir(m.configPath)
	if strings.TrimSpace(baseDir) == "" || baseDir == "." {
		baseDir = "./runtime"
	}
	access := strings.TrimSpace(os.Getenv("XRAY_ACCESS_LOG"))
	if access == "" {
		access = filepath.Join(baseDir, "xray-access.log")
	}
	errPath := strings.TrimSpace(os.Getenv("XRAY_ERROR_LOG"))
	if errPath == "" {
		errPath = filepath.Join(baseDir, "xray-error.log")
	}
	return xrayLogPaths{Access: access, Error: errPath}
}

func detectDefaultXrayBin() string {
	candidates := []string{
		"./bin/xray",
		"../bin/xray",
		"/opt/z-ui/bin/xray",
		"xray",
	}
	for _, one := range candidates {
		if one == "xray" {
			continue
		}
		if info, err := os.Stat(one); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return one
		}
	}
	return "xray"
}

func (m *XrayManager) GetStatus(c *gin.Context) {
	status, err := m.Snapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (m *XrayManager) GetConfig(c *gin.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"xrayPath":    m.xrayPath,
		"configPath":  m.configPath,
		"controlMode": m.controlMode,
		"serviceName": m.serviceName,
		"apiAddr":     m.apiAddr,
	})
}

func (m *XrayManager) UpdateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if req.XrayPath != "" {
		m.xrayPath = req.XrayPath
	}
	if req.ConfigPath != "" {
		m.configPath = req.ConfigPath
	}
	if req.APIAddr != "" {
		m.apiAddr = req.APIAddr
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "xray config updated",
		"xrayPath":    m.xrayPath,
		"configPath":  m.configPath,
		"controlMode": m.controlMode,
		"serviceName": m.serviceName,
		"apiAddr":     m.apiAddr,
	})
}

func (m *XrayManager) StatsOverview(c *gin.Context) {
	overview, err := m.QueryStatsOverview()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, overview)
}

func (m *XrayManager) SyncUsage(c *gin.Context) {
	updated, err := m.SyncInboundUsageFromStats()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "usage synced", "updatedCount": updated})
}

func (m *XrayManager) LimitPreview(c *gin.Context) {
	inbounds, err := m.store.ListEnabledInbounds()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type item struct {
		ID       int64    `json:"id"`
		Tag      string   `json:"tag"`
		Remark   string   `json:"remark"`
		Blocked  bool     `json:"blocked"`
		Reasons  []string `json:"reasons"`
		UsedGB   int64    `json:"usedGB"`
		TotalGB  int64    `json:"totalGB"`
		ExpiryAt *string  `json:"expiryAt,omitempty"`
	}
	out := make([]item, 0, len(inbounds))
	for _, inb := range inbounds {
		reasons := inboundBlockReasons(inb)
		var expiryAt *string
		if inb.ExpiryAt != nil {
			s := inb.ExpiryAt.UTC().Format(time.RFC3339)
			expiryAt = &s
		}
		out = append(out, item{
			ID:       inb.ID,
			Tag:      inb.Tag,
			Remark:   inb.Remark,
			Blocked:  len(reasons) > 0,
			Reasons:  reasons,
			UsedGB:   inb.UsedGB,
			TotalGB:  inb.TotalGB,
			ExpiryAt: expiryAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (m *XrayManager) ApplyConfig(c *gin.Context) {
	var req applyConfigRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
			return
		}
	}
	restart := true
	if req.Restart != nil {
		restart = *req.Restart
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, count, err := m.buildXrayConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("create config dir failed: %v", err)})
		return
	}
	if err := os.WriteFile(m.configPath, cfg, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("write config failed: %v", err)})
		return
	}

	if restart {
		if err := m.restartUnsafe(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("config written but restart failed: %v", err)})
			return
		}
	}
	recordAudit(c, m.store, "xray.apply_config", m.configPath, fmt.Sprintf("inbounds=%d restart=%v", count, restart))

	c.JSON(http.StatusOK, gin.H{
		"message":        "xray config applied",
		"configPath":     m.configPath,
		"inboundCount":   count,
		"restartApplied": restart,
	})
}

func (m *XrayManager) Start(c *gin.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.startUnsafe(); err != nil {
		if errors.Is(err, errAlreadyRunning) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, pid, _ := m.runningUnsafe()
	recordAudit(c, m.store, "xray.start", strconv.Itoa(pid), "")
	c.JSON(http.StatusOK, gin.H{"message": "xray started", "pid": pid})
}

func (m *XrayManager) Stop(c *gin.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.stopUnsafe(); err != nil {
		if errors.Is(err, errNotRunning) {
			c.JSON(http.StatusConflict, gin.H{"error": "xray is not running"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, m.store, "xray.stop", "", "")

	c.JSON(http.StatusOK, gin.H{"message": "xray stopped"})
}

func (m *XrayManager) Restart(c *gin.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.restartUnsafe(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_, pid, _ := m.runningUnsafe()
	recordAudit(c, m.store, "xray.restart", strconv.Itoa(pid), "")
	c.JSON(http.StatusOK, gin.H{"message": "xray restarted", "pid": pid})
}

var (
	errNotRunning     = errors.New("xray not running")
	errAlreadyRunning = errors.New("xray already running")
)

func (m *XrayManager) startUnsafe() error {
	running, _, err := m.runningUnsafe()
	if err != nil {
		return err
	}
	if running {
		return errAlreadyRunning
	}

	if m.controlMode == "systemd" {
		return runSystemctl("start", m.serviceName)
	}

	cmd := exec.Command(m.xrayPath, "run", "-c", m.configPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	m.cmd = cmd
	return nil
}

func (m *XrayManager) stopUnsafe() error {
	running, _, err := m.runningUnsafe()
	if err != nil {
		return err
	}
	if !running {
		m.cmd = nil
		return errNotRunning
	}

	if m.controlMode == "systemd" {
		return runSystemctl("stop", m.serviceName)
	}

	if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	if _, err := m.cmd.Process.Wait(); err != nil {
		return err
	}
	m.cmd = nil
	return nil
}

func (m *XrayManager) restartUnsafe() error {
	if m.controlMode == "systemd" {
		return runSystemctl("restart", m.serviceName)
	}

	if err := m.stopUnsafe(); err != nil && !errors.Is(err, errNotRunning) {
		return err
	}
	cmd := exec.Command(m.xrayPath, "run", "-c", m.configPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	m.cmd = cmd
	return nil
}

func (m *XrayManager) runningUnsafe() (bool, int, error) {
	if m.controlMode == "systemd" {
		out, err := exec.Command("systemctl", "is-active", m.serviceName).CombinedOutput()
		if err != nil {
			status := strings.TrimSpace(string(out))
			if status == "inactive" || status == "failed" || status == "unknown" || status == "" {
				return false, 0, nil
			}
			return false, 0, fmt.Errorf("systemctl is-active failed: %s", strings.TrimSpace(string(out)))
		}

		pidOut, err := exec.Command("systemctl", "show", "-p", "MainPID", "--value", m.serviceName).CombinedOutput()
		if err != nil {
			return true, 0, nil
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(pidOut)))
		return true, pid, nil
	}

	if m.cmd == nil || m.cmd.Process == nil {
		return false, 0, nil
	}
	if m.cmd.ProcessState != nil && m.cmd.ProcessState.Exited() {
		return false, 0, nil
	}
	if err := m.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false, 0, nil
	}
	return true, m.cmd.Process.Pid, nil
}

func runSystemctl(action, service string) error {
	out, err := exec.Command("systemctl", action, service).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s %s failed: %s", action, service, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *XrayManager) buildXrayConfig() ([]byte, int, error) {
	_, _ = m.SyncInboundUsageFromStats()
	inbounds, err := m.store.ListEnabledInbounds()
	if err != nil {
		return nil, 0, err
	}

	resultInbounds := make([]map[string]any, 0, len(inbounds)+1)
	for _, inb := range inbounds {
		if reasons := inboundBlockReasons(inb); len(reasons) > 0 {
			continue
		}

		settings, err := parseJSONObject(inb.SettingsJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("inbound %d settings json invalid: %w", inb.ID, err)
		}
		stream, err := parseJSONObject(inb.StreamSettingsJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("inbound %d streamSettings json invalid: %w", inb.ID, err)
		}
		sniffing, err := parseJSONObject(inb.SniffingJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("inbound %d sniffing json invalid: %w", inb.ID, err)
		}
		fallbacks, err := parseJSONArray(inb.FallbacksJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("inbound %d fallbacks json invalid: %w", inb.ID, err)
		}
		sockopt, err := parseJSONObject(inb.SockoptJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("inbound %d sockopt json invalid: %w", inb.ID, err)
		}
		httpObfs, err := parseJSONObject(inb.HTTPObfsJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("inbound %d httpObfs json invalid: %w", inb.ID, err)
		}
		externalProxy, err := parseJSONObject(inb.ExternalProxyJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("inbound %d externalProxy json invalid: %w", inb.ID, err)
		}

		if inb.Authentication != "" {
			settings["auth"] = inb.Authentication
		}
		if inb.Decryption != "" {
			settings["decryption"] = inb.Decryption
		}
		if inb.Encryption != "" {
			settings["encryption"] = inb.Encryption
		}

		entry := map[string]any{
			"tag":      firstNonEmpty(inb.Tag, fmt.Sprintf("inbound-%d-%s", inb.ID, inb.Remark)),
			"listen":   firstNonEmpty(inb.Listen, "0.0.0.0"),
			"port":     inb.Port,
			"protocol": inb.Protocol,
			"settings": settings,
		}

		stream["network"] = firstNonEmpty(inb.Transport, streamString(stream, "network", "tcp"))
		stream["security"] = firstNonEmpty(inb.Security, streamString(stream, "security", "none"))
		if len(sockopt) > 0 || inb.ProxyProtocol {
			sock := map[string]any{}
			if old, ok := stream["sockopt"].(map[string]any); ok {
				mergeMap(sock, old)
			}
			mergeMap(sock, sockopt)
			if inb.ProxyProtocol {
				sock["acceptProxyProtocol"] = true
			}
			if len(externalProxy) > 0 {
				if tag, ok := externalProxy["tag"].(string); ok && strings.TrimSpace(tag) != "" {
					sock["dialerProxy"] = tag
				}
			}
			stream["sockopt"] = sock
		}
		if len(httpObfs) > 0 {
			stream["tcpSettings"] = map[string]any{"header": httpObfs}
		}

		if len(stream) > 0 {
			entry["streamSettings"] = stream
		}
		if len(sniffing) > 0 {
			entry["sniffing"] = sniffing
		}
		if len(fallbacks) > 0 {
			entry["fallbacks"] = fallbacks
		}
		resultInbounds = append(resultInbounds, entry)
	}

	// API inbound for xray-core command services (stats/handler/logger).
	resultInbounds = append(resultInbounds, map[string]any{
		"tag":      "api",
		"listen":   "127.0.0.1",
		"port":     10085,
		"protocol": "dokodemo-door",
		"settings": map[string]any{"address": "127.0.0.1"},
	})

	logs := m.defaultLogPaths()
	cfg := map[string]any{
		"api": map[string]any{
			"tag":      "api",
			"services": []string{"StatsService", "HandlerService", "LoggerService"},
		},
		"log": map[string]any{
			"loglevel": "warning",
			"access":   logs.Access,
			"error":    logs.Error,
		},
		"stats": map[string]any{},
		"policy": map[string]any{
			"system": map[string]any{
				"statsInboundUplink":   true,
				"statsInboundDownlink": true,
				"statsUserUplink":      true,
				"statsUserDownlink":    true,
			},
		},
		"inbounds": resultInbounds,
		"outbounds": []map[string]any{
			{"protocol": "freedom", "tag": "direct"},
			{"protocol": "blackhole", "tag": "blocked"},
		},
		"routing": map[string]any{
			"rules": []map[string]any{
				{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"},
			},
		},
	}

	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	return encoded, len(resultInbounds) - 1, nil
}

func parseJSONObject(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return map[string]any{}, nil
	}
	return obj, nil
}

func parseJSONArray(raw string) ([]any, error) {
	if strings.TrimSpace(raw) == "" {
		return []any{}, nil
	}
	var arr []any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil, err
	}
	if arr == nil {
		return []any{}, nil
	}
	return arr, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func mergeMap(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

func streamString(m map[string]any, key, fallback string) string {
	v, ok := m[key]
	if !ok {
		return fallback
	}
	s, _ := v.(string)
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func inboundBlockReasons(inb storage.Inbound) []string {
	reasons := make([]string, 0)
	if inb.TotalGB > 0 && inb.UsedGB >= inb.TotalGB {
		reasons = append(reasons, "traffic_limit_reached")
	}
	if inb.ExpiryAt != nil && time.Now().After(*inb.ExpiryAt) {
		reasons = append(reasons, "expired")
	}
	if inb.DeviceLimit > 0 {
		clientCount := clientCountFromSettings(inb.Protocol, inb.SettingsJSON)
		if clientCount > 0 && clientCount > inb.DeviceLimit {
			reasons = append(reasons, "device_limit_exceeded_by_clients")
		}
	}
	return reasons
}

func clientCountFromSettings(protocol, settingsJSON string) int {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return 0
	}
	if protocol == "shadowsocks" || protocol == "shadowsocks-2022" || protocol == "shadowsocks_2022" {
		if users, ok := settings["users"].([]any); ok {
			return len(users)
		}
		if _, ok := settings["password"].(string); ok {
			return 1
		}
		return 0
	}
	if clients, ok := settings["clients"].([]any); ok {
		return len(clients)
	}
	return 0
}

func (m *XrayManager) Snapshot() (XrayStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	running, pid, err := m.runningUnsafe()
	if err != nil {
		return XrayStatus{}, err
	}
	return XrayStatus{
		Running:     running,
		PID:         pid,
		XrayPath:    m.xrayPath,
		ConfigPath:  m.configPath,
		ControlMode: m.controlMode,
		ServiceName: m.serviceName,
		APIAddr:     m.apiAddr,
	}, nil
}
