package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type LogHandler struct {
	xray *XrayManager
}

func NewLogHandler(xray *XrayManager) *LogHandler {
	return &LogHandler{xray: xray}
}

func (h *LogHandler) Xray(c *gin.Context) {
	lines := clampLines(c.Query("lines"))
	target := strings.ToLower(strings.TrimSpace(c.DefaultQuery("target", "auto")))

	if target == "journal" {
		content, source, err := h.readXrayJournal(lines)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"source": source, "lines": lines, "content": content, "updatedAt": time.Now()})
		return
	}

	path, source := h.resolveXrayLogPath(target)
	if path != "" {
		content, err := tailFileLines(path, lines)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"source": source, "lines": lines, "content": content, "updatedAt": time.Now()})
			return
		}
	}

	content, source, err := h.readXrayJournal(lines)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "无法读取 Xray 日志，请配置 XRAY_ERROR_LOG / XRAY_ACCESS_LOG 或启用 journalctl"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"source": source, "lines": lines, "content": content, "updatedAt": time.Now()})
}

func (h *LogHandler) System(c *gin.Context) {
	lines := clampLines(c.Query("lines"))

	if path := strings.TrimSpace(os.Getenv("ZUI_SYSTEM_LOG")); path != "" {
		content, err := tailFileLines(path, lines)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"source": path, "lines": lines, "content": content, "updatedAt": time.Now()})
			return
		}
	}

	if content, err := readJournal(lines, ""); err == nil {
		c.JSON(http.StatusOK, gin.H{"source": "journalctl", "lines": lines, "content": content, "updatedAt": time.Now()})
		return
	}

	for _, path := range defaultSystemLogFiles() {
		content, err := tailFileLines(path, lines)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"source": path, "lines": lines, "content": content, "updatedAt": time.Now()})
			return
		}
	}

	c.JSON(http.StatusBadGateway, gin.H{"error": "无法读取系统日志（请检查日志文件权限）"})
}

func (h *LogHandler) resolveXrayLogPath(target string) (string, string) {
	if target == "error" {
		if path := strings.TrimSpace(os.Getenv("XRAY_ERROR_LOG")); path != "" {
			return path, path
		}
	}
	if target == "access" {
		if path := strings.TrimSpace(os.Getenv("XRAY_ACCESS_LOG")); path != "" {
			return path, path
		}
	}

	if target == "auto" || target == "error" || target == "access" {
		if cfgPath := strings.TrimSpace(h.xray.configPath); cfgPath != "" {
			if logCfg, err := parseXrayLogConfig(cfgPath); err == nil {
				if target == "access" && strings.TrimSpace(logCfg.Access) != "" {
					return logCfg.Access, logCfg.Access
				}
				if target == "error" && strings.TrimSpace(logCfg.Error) != "" {
					return logCfg.Error, logCfg.Error
				}
				if target == "auto" {
					if strings.TrimSpace(logCfg.Error) != "" {
						return logCfg.Error, logCfg.Error
					}
					if strings.TrimSpace(logCfg.Access) != "" {
						return logCfg.Access, logCfg.Access
					}
				}
			}
		}
	}

	if target == "auto" || target == "error" {
		if path := strings.TrimSpace(os.Getenv("XRAY_ERROR_LOG")); path != "" {
			return path, path
		}
	}
	if target == "auto" || target == "access" {
		if path := strings.TrimSpace(os.Getenv("XRAY_ACCESS_LOG")); path != "" {
			return path, path
		}
	}

	defaults := h.xray.defaultLogPaths()
	if target == "error" {
		return defaults.Error, defaults.Error
	}
	if target == "access" {
		return defaults.Access, defaults.Access
	}
	if target == "auto" {
		return defaults.Error, defaults.Error
	}

	return "", ""
}

func (h *LogHandler) readXrayJournal(lines int) (string, string, error) {
	service := strings.TrimSpace(h.xray.serviceName)
	if service == "" {
		service = "xray"
	}
	content, err := readJournal(lines, service)
	if err != nil {
		return "", "", err
	}
	return content, "journalctl -u " + service, nil
}

func defaultSystemLogFiles() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/var/log/system.log"}
	case "linux":
		return []string{"/var/log/syslog", "/var/log/messages"}
	default:
		return nil
	}
}

func readJournal(lines int, unit string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	args := []string{"-n", strconv.Itoa(lines), "--no-pager"}
	if unit != "" {
		args = append([]string{"-u", unit}, args...)
	}
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", errors.New("journalctl timeout")
	}
	if err != nil {
		return "", err
	}
	return string(output), nil
}

type xrayLogConfig struct {
	Access string `json:"access"`
	Error  string `json:"error"`
}

func parseXrayLogConfig(configPath string) (xrayLogConfig, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return xrayLogConfig{}, err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return xrayLogConfig{}, err
	}
	section, ok := root["log"]
	if !ok {
		return xrayLogConfig{}, errors.New("missing log section")
	}
	var cfg xrayLogConfig
	if err := json.Unmarshal(section, &cfg); err != nil {
		return xrayLogConfig{}, err
	}
	return cfg, nil
}

func tailFileLines(path string, n int) (string, error) {
	if n <= 0 {
		n = 200
	}
	clean := filepath.Clean(path)
	file, err := os.Open(clean)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	ring := make([]string, n)
	count := 0
	for scanner.Scan() {
		ring[count%n] = scanner.Text()
		count++
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if count == 0 {
		return "", nil
	}

	start := 0
	if count > n {
		start = count % n
		count = n
	}
	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		idx := (start + i) % len(ring)
		lines = append(lines, ring[idx])
	}
	return strings.Join(lines, "\n"), nil
}

func clampLines(raw string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(raw))
	if v <= 0 {
		return 200
	}
	if v > 5000 {
		return 5000
	}
	return v
}
