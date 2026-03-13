package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ToolsHandler struct{}

func NewToolsHandler() *ToolsHandler {
	return &ToolsHandler{}
}

type bbrEnableRequest struct {
	Qdisc      string `json:"qdisc"`
	Congestion string `json:"congestion"`
}

func (h *ToolsHandler) BBRStatus(c *gin.Context) {
	status := queryBBRStatus()
	c.JSON(http.StatusOK, status)
}

func (h *ToolsHandler) BBREnable(c *gin.Context) {
	if runtime.GOOS != "linux" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BBR 仅支持 Linux"})
		return
	}

	var req bbrEnableRequest
	_ = c.ShouldBindJSON(&req)
	qdisc := strings.TrimSpace(req.Qdisc)
	if qdisc == "" {
		qdisc = "fq"
	}
	congestion := strings.TrimSpace(req.Congestion)
	if congestion == "" {
		congestion = "bbr"
	}

	if err := setSysctl("net.core.default_qdisc", qdisc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := setSysctl("net.ipv4.tcp_congestion_control", congestion); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	confText := fmt.Sprintf("net.core.default_qdisc=%s\nnet.ipv4.tcp_congestion_control=%s\n", qdisc, congestion)
	confPath := strings.TrimSpace(os.Getenv("ZUI_BBR_SYSCTL_PATH"))
	if confPath == "" {
		confPath = "/etc/sysctl.d/99-z-ui-bbr.conf"
	}
	persisted := false
	if err := os.WriteFile(confPath, []byte(confText), 0o644); err == nil {
		persisted = true
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "bbr enabled",
		"persisted":  persisted,
		"configPath": confPath,
		"status":     queryBBRStatus(),
	})
}

func (h *ToolsHandler) Speedtest(c *gin.Context) {
	var body struct {
		URL string `json:"url"`
	}
	_ = c.ShouldBindJSON(&body)
	targetURL := strings.TrimSpace(body.URL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(c.Query("url"))
	}
	if targetURL == "" {
		targetURL = "https://speed.cloudflare.com/__down?bytes=10000000"
	}

	result, err := runSpeedtest(targetURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func queryBBRStatus() gin.H {
	if runtime.GOOS != "linux" {
		return gin.H{
			"os":        runtime.GOOS,
			"supported": false,
			"enabled":   false,
		}
	}

	qdisc, _ := readSysctl("net.core.default_qdisc")
	cc, _ := readSysctl("net.ipv4.tcp_congestion_control")
	available, _ := readSysctl("net.ipv4.tcp_available_congestion_control")
	modLoaded, _ := readSysctl("net.ipv4.tcp_allowed_congestion_control")

	enabled := strings.EqualFold(strings.TrimSpace(cc), "bbr")
	availableLower := strings.ToLower(available + " " + modLoaded)
	loaded := strings.Contains(availableLower, "bbr")

	return gin.H{
		"os":                  runtime.GOOS,
		"supported":           true,
		"enabled":             enabled,
		"qdisc":               strings.TrimSpace(qdisc),
		"congestionControl":   strings.TrimSpace(cc),
		"availableCongestion": strings.TrimSpace(available),
		"bbrAvailable":        loaded,
	}
}

func readSysctl(key string) (string, error) {
	out, err := exec.Command("sysctl", "-n", key).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read sysctl %s failed: %s", key, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func setSysctl(key, value string) error {
	if os.Geteuid() != 0 {
		return errors.New("需要 root 权限执行 sysctl")
	}
	out, err := exec.Command("sysctl", "-w", fmt.Sprintf("%s=%s", key, value)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("set sysctl %s failed: %s", key, strings.TrimSpace(string(out)))
	}
	return nil
}

func runSpeedtest(rawURL string) (gin.H, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("测速 URL 不合法")
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		if strings.EqualFold(parsed.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}

	dialStart := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连通性测试失败: %w", err)
	}
	dialMs := time.Since(dialStart).Milliseconds()
	_ = conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("测速请求失败: %s", resp.Status)
	}

	limitMB := 20
	if v := strings.TrimSpace(os.Getenv("ZUI_SPEEDTEST_MAX_MB")); v != "" {
		if n, parseErr := strconv.Atoi(v); parseErr == nil && n > 0 {
			limitMB = n
		}
	}
	limitBytes := int64(limitMB) * 1024 * 1024
	n, err := io.CopyN(io.Discard, resp.Body, limitBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	duration := time.Since(start)
	sec := duration.Seconds()
	if sec <= 0 {
		sec = 0.001
	}
	mbps := (float64(n) * 8 / 1_000_000) / sec

	return gin.H{
		"url":          rawURL,
		"dialMs":       dialMs,
		"durationMs":   duration.Milliseconds(),
		"downloadedMB": float64(n) / 1024.0 / 1024.0,
		"speedMbps":    mbps,
		"cappedAtMB":   limitMB,
		"testedAt":     time.Now(),
	}, nil
}
