package cli

import (
	"archive/zip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"zui/storage"
)

func Run(args []string) (handled bool, err error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage()
		return true, nil
	case "status":
		return true, cmdStatus()
	case "init":
		publicBaseURL := ""
		if len(args) > 1 {
			publicBaseURL = args[1]
		}
		return true, cmdInit(publicBaseURL)
	case "set-user":
		if len(args) < 2 {
			return true, errors.New("usage: z-ui set-user <username>")
		}
		return true, cmdSetUser(args[1])
	case "set-pass":
		if len(args) < 2 {
			return true, errors.New("usage: z-ui set-pass <password>")
		}
		return true, cmdSetPass(args[1])
	case "ssl":
		return true, cmdSSL(args[1:])
	case "speedtest":
		url := "https://speed.cloudflare.com/__down?bytes=10000000"
		if len(args) > 1 {
			url = args[1]
		}
		return true, cmdSpeedtest(url)
	case "bbr":
		return true, cmdBBR(args[1:])
	case "logs":
		return true, cmdLogs(args[1:])
	case "backup":
		output := ""
		if len(args) > 1 {
			output = strings.TrimSpace(args[1])
		}
		return true, cmdBackup(output)
	case "restore":
		if len(args) < 2 {
			return true, errors.New("usage: z-ui restore <backup.zip>")
		}
		return true, cmdRestore(args[1])
	case "doctor":
		return true, cmdDoctor()
	default:
		return true, fmt.Errorf("unknown command: %s", args[0])
	}
}

func cmdBBR(args []string) error {
	if len(args) == 0 || args[0] == "status" {
		return cmdBBRStatus()
	}
	if args[0] == "enable" {
		return cmdBBREnable()
	}
	return errors.New("usage: z-ui bbr [status|enable]")
}

func cmdBBRStatus() error {
	if runtime.GOOS != "linux" {
		fmt.Printf("os: %s\n", runtime.GOOS)
		fmt.Println("bbr: unsupported")
		return nil
	}

	qdisc, _ := readSysctlValue("net.core.default_qdisc")
	cc, _ := readSysctlValue("net.ipv4.tcp_congestion_control")
	available, _ := readSysctlValue("net.ipv4.tcp_available_congestion_control")

	fmt.Println("BBR Status")
	fmt.Println("----------")
	fmt.Printf("qdisc               : %s\n", qdisc)
	fmt.Printf("congestion control  : %s\n", cc)
	fmt.Printf("available controls  : %s\n", available)
	fmt.Printf("enabled             : %v\n", strings.EqualFold(strings.TrimSpace(cc), "bbr"))
	return nil
}

func cmdBBREnable() error {
	if runtime.GOOS != "linux" {
		return errors.New("bbr enable only supports linux")
	}
	if os.Geteuid() != 0 {
		return errors.New("need root privileges to enable bbr")
	}
	if err := writeSysctlValue("net.core.default_qdisc", "fq"); err != nil {
		return err
	}
	if err := writeSysctlValue("net.ipv4.tcp_congestion_control", "bbr"); err != nil {
		return err
	}
	confPath := envOrDefault("ZUI_BBR_SYSCTL_PATH", "/etc/sysctl.d/99-z-ui-bbr.conf")
	content := "net.core.default_qdisc=fq\nnet.ipv4.tcp_congestion_control=bbr\n"
	if err := os.WriteFile(confPath, []byte(content), 0o644); err != nil {
		fmt.Printf("warn: failed to persist %s: %v\n", confPath, err)
	} else {
		fmt.Printf("persisted config: %s\n", confPath)
	}
	fmt.Println("bbr enabled")
	return cmdBBRStatus()
}

func cmdLogs(args []string) error {
	source := "xray"
	lines := 200
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		source = strings.ToLower(strings.TrimSpace(args[0]))
	}
	if len(args) > 1 {
		if n, err := strconv.Atoi(strings.TrimSpace(args[1])); err == nil && n > 0 {
			lines = n
		}
	}

	var target string
	switch source {
	case "xray":
		target = envOrDefault("XRAY_ERROR_LOG", "./runtime/xray-error.log")
	case "system":
		target = envOrDefault("ZUI_SYSTEM_LOG", "/var/log/syslog")
	case "audit":
		store, err := storage.OpenDefaultStore()
		if err != nil {
			return err
		}
		defer store.Close()
		items, err := store.ListAuditLogs(lines, 0)
		if err != nil {
			return err
		}
		for _, item := range items {
			fmt.Printf("[%s] user=%s ip=%s action=%s target=%s detail=%s\n",
				item.CreatedAt.Format(time.RFC3339), item.Username, item.IP, item.Action, item.Target, item.Detail)
		}
		return nil
	default:
		return errors.New("usage: z-ui logs [xray|system|audit] [lines]")
	}

	if err := printTail(target, lines); err != nil {
		if source == "xray" {
			return fmt.Errorf("read %s failed (try: journalctl -u xray -n %d --no-pager): %w", target, lines, err)
		}
		return err
	}
	return nil
}

func cmdStatus() error {
	store, err := storage.OpenDefaultStore()
	if err != nil {
		return err
	}
	defer store.Close()

	settings, err := store.GetPanelSettings()
	if err != nil {
		return err
	}
	total, enabled, err := store.CountInbounds()
	if err != nil {
		return err
	}

	xrayControl := os.Getenv("XRAY_CONTROL")
	if xrayControl == "" {
		xrayControl = "process"
	}
	xrayService := os.Getenv("XRAY_SERVICE")
	if xrayService == "" {
		xrayService = "xray"
	}
	xrayConfig := os.Getenv("XRAY_CONFIG")
	if xrayConfig == "" {
		xrayConfig = "/usr/local/etc/xray/config.json"
	}

	fmt.Println("Z-UI Status")
	fmt.Println("-----------")
	fmt.Printf("panel title: %s\n", settings.Title)
	fmt.Printf("admin user: %s\n", settings.AdminUsername)
	fmt.Printf("public base: %s\n", settings.PublicBaseURL)
	fmt.Printf("db path: %s\n", envOrDefault("ZUI_DB", "./zui.db"))
	fmt.Printf("inbounds: total=%d enabled=%d\n", total, enabled)
	fmt.Printf("xray control: %s\n", xrayControl)
	if xrayControl == "systemd" {
		fmt.Printf("xray service: %s (%s)\n", xrayService, systemdState(xrayService))
	} else {
		fmt.Printf("xray bin: %s\n", envOrDefault("XRAY_BIN", "xray"))
	}
	fmt.Printf("xray config: %s\n", xrayConfig)
	return nil
}

type backupManifest struct {
	CreatedAt time.Time         `json:"createdAt"`
	Files     map[string]string `json:"files"`
}

func cmdBackup(output string) error {
	if output == "" {
		output = fmt.Sprintf("./z-ui-backup-%s.zip", time.Now().Format("20060102-150405"))
	}
	if !filepath.IsAbs(output) {
		cwd, _ := os.Getwd()
		output = filepath.Join(cwd, output)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}

	files := map[string]string{
		"db/zui.db":                 envOrDefault("ZUI_DB", "./zui.db"),
		"runtime/xray-config.json":  envOrDefault("XRAY_CONFIG", "./runtime/xray-config.json"),
		"runtime/xray-access.log":   envOrDefault("XRAY_ACCESS_LOG", "./runtime/xray-access.log"),
		"runtime/xray-error.log":    envOrDefault("XRAY_ERROR_LOG", "./runtime/xray-error.log"),
	}

	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	manifest := backupManifest{CreatedAt: time.Now(), Files: map[string]string{}}
	count := 0
	for entryName, sourcePath := range files {
		sourcePath = strings.TrimSpace(sourcePath)
		if sourcePath == "" {
			continue
		}
		clean := filepath.Clean(sourcePath)
		stat, err := os.Stat(clean)
		if err != nil || stat.IsDir() {
			continue
		}
		fr, err := os.Open(clean)
		if err != nil {
			continue
		}
		w, err := zw.Create(entryName)
		if err != nil {
			_ = fr.Close()
			continue
		}
		if _, err := io.Copy(w, fr); err == nil {
			manifest.Files[entryName] = clean
			count++
		}
		_ = fr.Close()
	}

	mw, err := zw.Create("manifest.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(mw).Encode(manifest); err != nil {
		return err
	}

	fmt.Printf("backup created: %s\n", output)
	fmt.Printf("files packed: %d\n", count)
	return nil
}

func cmdRestore(archivePath string) error {
	archivePath = strings.TrimSpace(archivePath)
	if archivePath == "" {
		return errors.New("backup path cannot be empty")
	}
	if !filepath.IsAbs(archivePath) {
		cwd, _ := os.Getwd()
		archivePath = filepath.Join(cwd, archivePath)
	}

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	target := map[string]string{
		"db/zui.db":                envOrDefault("ZUI_DB", "./zui.db"),
		"runtime/xray-config.json": envOrDefault("XRAY_CONFIG", "./runtime/xray-config.json"),
		"runtime/xray-access.log":  envOrDefault("XRAY_ACCESS_LOG", "./runtime/xray-access.log"),
		"runtime/xray-error.log":   envOrDefault("XRAY_ERROR_LOG", "./runtime/xray-error.log"),
	}

	restored := 0
	for _, f := range zr.File {
		dst, ok := target[f.Name]
		if !ok {
			continue
		}
		r, err := f.Open()
		if err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			_ = r.Close()
			continue
		}
		w, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			_ = r.Close()
			continue
		}
		if _, err := io.Copy(w, r); err == nil {
			restored++
		}
		_ = w.Close()
		_ = r.Close()
	}

	fmt.Printf("backup restored from: %s\n", archivePath)
	fmt.Printf("files restored: %d\n", restored)
	if restored == 0 {
		return errors.New("no known files restored from backup")
	}
	return nil
}

func cmdDoctor() error {
	type check struct {
		Name   string
		Level  string
		Detail string
	}
	checks := make([]check, 0)
	add := func(name, level, detail string) {
		checks = append(checks, check{Name: name, Level: level, Detail: detail})
	}

	store, err := storage.OpenDefaultStore()
	if err != nil {
		add("database", "FAIL", err.Error())
	} else {
		_ = store.Close()
		add("database", "PASS", envOrDefault("ZUI_DB", "./zui.db"))
	}

	port := envOrDefault("PORT", "8081")
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		add("port", "WARN", fmt.Sprintf("%s unavailable: %v", port, err))
	} else {
		_ = ln.Close()
		add("port", "PASS", fmt.Sprintf("%s available", port))
	}

	xrayControl := envOrDefault("XRAY_CONTROL", "process")
	if xrayControl == "systemd" {
		service := envOrDefault("XRAY_SERVICE", "xray")
		state := systemdState(service)
		if state == "active" {
			add("xray service", "PASS", service+" is active")
		} else {
			add("xray service", "WARN", service+" state: "+state)
		}
	} else {
		xrayBin := envOrDefault("XRAY_BIN", "xray")
		if _, err := exec.LookPath(xrayBin); err != nil {
			if _, statErr := os.Stat(xrayBin); statErr != nil {
				add("xray binary", "FAIL", "not found: "+xrayBin)
			} else {
				add("xray binary", "PASS", xrayBin)
			}
		} else {
			add("xray binary", "PASS", xrayBin)
		}
	}

	xrayConfig := envOrDefault("XRAY_CONFIG", "./runtime/xray-config.json")
	if err := os.MkdirAll(filepath.Dir(xrayConfig), 0o755); err != nil {
		add("xray config dir", "FAIL", err.Error())
	} else {
		add("xray config dir", "PASS", filepath.Dir(xrayConfig))
	}

	frontDir := envOrDefault("ZUI_FRONT_DIR", "../front/dist")
	if stat, err := os.Stat(frontDir); err != nil || !stat.IsDir() {
		add("frontend", "WARN", "not found: "+frontDir)
	} else {
		add("frontend", "PASS", frontDir)
	}

	if strings.TrimSpace(os.Getenv("PANEL_PASSWORD")) == "admin" {
		add("panel password", "WARN", "PANEL_PASSWORD still admin")
	}

	failCount := 0
	warnCount := 0
	fmt.Println("Z-UI Doctor")
	fmt.Println("-----------")
	for _, one := range checks {
		fmt.Printf("[%s] %s: %s\n", one.Level, one.Name, one.Detail)
		if one.Level == "FAIL" {
			failCount++
		}
		if one.Level == "WARN" {
			warnCount++
		}
	}
	fmt.Printf("summary: fail=%d warn=%d\n", failCount, warnCount)
	if failCount > 0 {
		return errors.New("doctor found critical issues")
	}
	return nil
}

func cmdInit(publicBaseURL string) error {
	store, err := storage.OpenDefaultStore()
	if err != nil {
		return err
	}
	defer store.Close()

	settings, err := store.GetPanelSettings()
	if err != nil {
		return err
	}

	randomUser, err := randomToken(4)
	if err != nil {
		return err
	}
	randomPass, err := randomToken(12)
	if err != nil {
		return err
	}
	username := "admin-" + strings.ToLower(randomUser)
	password := randomPass

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	baseURL := strings.TrimSpace(publicBaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(settings.PublicBaseURL)
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("PANEL_PUBLIC_BASE"))
	}
	if baseURL == "" {
		baseURL = "http://127.0.0.1:" + envOrDefault("PORT", "8081")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	settings.AdminUsername = username
	settings.AdminPasswordHash = string(hash)
	settings.PublicBaseURL = baseURL

	if _, err := store.UpdatePanelSettings(settings); err != nil {
		return err
	}

	fmt.Println("panel init completed")
	fmt.Printf("panel url : %s/login.html\n", baseURL)
	fmt.Printf("username  : %s\n", username)
	fmt.Printf("password  : %s\n", password)
	return nil
}

func cmdSetUser(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username cannot be empty")
	}
	store, err := storage.OpenDefaultStore()
	if err != nil {
		return err
	}
	defer store.Close()

	cur, err := store.GetPanelSettings()
	if err != nil {
		return err
	}
	if cur.AdminPasswordHash == "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		cur.AdminPasswordHash = string(hash)
	}
	cur.AdminUsername = username
	_, err = store.UpdatePanelSettings(cur)
	if err != nil {
		return err
	}
	fmt.Printf("admin username updated: %s\n", username)
	return nil
}

func cmdSetPass(password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("password cannot be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	store, err := storage.OpenDefaultStore()
	if err != nil {
		return err
	}
	defer store.Close()

	cur, err := store.GetPanelSettings()
	if err != nil {
		return err
	}
	if strings.TrimSpace(cur.AdminUsername) == "" {
		cur.AdminUsername = "admin"
	}
	cur.AdminPasswordHash = string(hash)
	_, err = store.UpdatePanelSettings(cur)
	if err != nil {
		return err
	}
	fmt.Println("admin password updated")
	return nil
}

func cmdSSL(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: z-ui ssl <self-signed|issue> ...")
	}
	switch args[0] {
	case "self-signed":
		if len(args) < 2 {
			return errors.New("usage: z-ui ssl self-signed <domain> [certPath] [keyPath]")
		}
		domain := args[1]
		certPath := "/etc/z-ui/ssl/z-ui.crt"
		keyPath := "/etc/z-ui/ssl/z-ui.key"
		if len(args) > 2 {
			certPath = args[2]
		}
		if len(args) > 3 {
			keyPath = args[3]
		}
		if err := generateSelfSignedCert(domain, certPath, keyPath); err != nil {
			return err
		}
		fmt.Printf("self-signed certificate generated:\ncert: %s\nkey : %s\n", certPath, keyPath)
		return nil
	case "issue":
		if len(args) < 2 {
			return errors.New("usage: z-ui ssl issue <domain> [email]")
		}
		email := ""
		if len(args) > 2 {
			email = strings.TrimSpace(args[2])
		}
		return cmdSSLIssue(args[1], email)
	case "renew":
		return cmdSSLRenew()
	default:
		return errors.New("usage: z-ui ssl <self-signed|issue|renew> ...")
	}
}

func cmdSSLIssue(domain, email string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return errors.New("domain cannot be empty")
	}

	if _, err := exec.LookPath("certbot"); err != nil {
		return errors.New("certbot not found, please install certbot first")
	}

	webroot := envOrDefault("ZUI_CERTBOT_WEBROOT", "/var/www/certbot")
	if err := os.MkdirAll(webroot, 0o755); err != nil {
		return fmt.Errorf("create webroot failed: %w", err)
	}

	args := []string{"certonly", "--webroot", "-w", webroot, "-d", domain, "--non-interactive", "--agree-tos"}
	if email != "" {
		args = append(args, "-m", email)
	} else {
		args = append(args, "--register-unsafely-without-email")
	}

	fmt.Printf("issuing certificate for %s with certbot...\n", domain)
	cmd := exec.Command("certbot", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("certbot issue failed: %w", err)
	}

	fullchain := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)
	privkey := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", domain)
	fmt.Println("certificate issued")
	fmt.Printf("fullchain: %s\n", fullchain)
	fmt.Printf("privkey  : %s\n", privkey)

	if envOrDefault("ZUI_SSL_RELOAD_NGINX", "0") == "1" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			_ = exec.Command("systemctl", "reload", "nginx").Run()
			fmt.Println("nginx reloaded")
		}
	}

	fmt.Println("next: ensure nginx ssl_certificate points to above files")
	return nil
}

func cmdSSLRenew() error {
	if _, err := exec.LookPath("certbot"); err != nil {
		return errors.New("certbot not found, please install certbot first")
	}

	fmt.Println("renewing certificates with certbot...")
	cmd := exec.Command("certbot", "renew")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("certbot renew failed: %w", err)
	}

	if envOrDefault("ZUI_SSL_RELOAD_NGINX", "1") == "1" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			if reloadErr := exec.Command("systemctl", "reload", "nginx").Run(); reloadErr == nil {
				fmt.Println("nginx reloaded")
			} else {
				fmt.Printf("warn: nginx reload failed: %v\n", reloadErr)
			}
		}
	}

	fmt.Println("renew done")
	return nil
}

func cmdSpeedtest(url string) error {
	start := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("speedtest http status: %s", resp.Status)
	}

	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return err
	}
	d := time.Since(start).Seconds()
	if d <= 0 {
		d = 0.001
	}
	mbits := (float64(n) * 8 / 1_000_000) / d
	fmt.Printf("speedtest url: %s\n", url)
	fmt.Printf("downloaded: %.2f MB\n", float64(n)/1024.0/1024.0)
	fmt.Printf("duration  : %.2f s\n", d)
	fmt.Printf("speed     : %.2f Mbps\n", mbits)
	return nil
}

func randomToken(n int) (string, error) {
	if n <= 0 {
		n = 8
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if len(token) > n {
		token = token[:n]
	}
	return token, nil
}

func generateSelfSignedCert(domain, certPath, keyPath string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   domain,
			Organization: []string{"Z-UI"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return err
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	keyBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return err
	}
	return nil
}

func systemdState(service string) string {
	out, err := exec.Command("systemctl", "is-active", service).CombinedOutput()
	if err != nil {
		t := strings.TrimSpace(string(out))
		if t == "" {
			return "unknown"
		}
		return t
	}
	return strings.TrimSpace(string(out))
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func printUsage() {
	fmt.Println("z-ui commands:")
	fmt.Println("  基础")
	fmt.Println("    z-ui status")
	fmt.Println("    z-ui doctor")
	fmt.Println("    z-ui init [publicBaseURL]")
	fmt.Println("    z-ui set-user <username>")
	fmt.Println("    z-ui set-pass <password>")
	fmt.Println("    z-ui backup [output.zip]")
	fmt.Println("    z-ui restore <backup.zip>")
	fmt.Println("  网络")
	fmt.Println("    z-ui speedtest [url]")
	fmt.Println("    z-ui bbr [status|enable]")
	fmt.Println("  日志")
	fmt.Println("    z-ui logs [xray|system|audit] [lines]")
	fmt.Println("  SSL")
	fmt.Println("    z-ui ssl self-signed <domain> [certPath] [keyPath]")
	fmt.Println("    z-ui ssl issue <domain> [email]")
	fmt.Println("    z-ui ssl renew")
}

func readSysctlValue(key string) (string, error) {
	out, err := exec.Command("sysctl", "-n", key).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read sysctl %s failed: %s", key, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func writeSysctlValue(key, value string) error {
	out, err := exec.Command("sysctl", "-w", fmt.Sprintf("%s=%s", key, value)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("set sysctl %s failed: %s", key, strings.TrimSpace(string(out)))
	}
	return nil
}

func printTail(path string, n int) error {
	if n <= 0 {
		n = 200
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	fmt.Printf("source: %s\n", path)
	fmt.Println(strings.Join(lines, "\n"))
	return nil
}
