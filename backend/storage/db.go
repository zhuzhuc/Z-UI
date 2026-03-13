package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Inbound struct {
	ID                 int64      `json:"id"`
	Tag                string     `json:"tag"`
	Remark             string     `json:"remark"`
	Protocol           string     `json:"protocol"`
	Listen             string     `json:"listen"`
	Port               int        `json:"port"`
	TotalGB            int64      `json:"totalGB"`
	UsedGB             int64      `json:"usedGB"`
	DeviceLimit        int        `json:"deviceLimit"`
	ExpiryAt           *time.Time `json:"expiryAt"`
	Authentication     string     `json:"authentication"`
	Decryption         string     `json:"decryption"`
	Encryption         string     `json:"encryption"`
	Transport          string     `json:"transport"`
	Security           string     `json:"security"`
	ProxyProtocol      bool       `json:"proxyProtocol"`
	SettingsJSON       string     `json:"settingsJson"`
	StreamSettingsJSON string     `json:"streamSettingsJson"`
	SniffingJSON       string     `json:"sniffingJson"`
	FallbacksJSON      string     `json:"fallbacksJson"`
	SockoptJSON        string     `json:"sockoptJson"`
	HTTPObfsJSON       string     `json:"httpObfsJson"`
	ExternalProxyJSON  string     `json:"externalProxyJson"`
	Enable             bool       `json:"enable"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type PanelSettings struct {
	Title                string `json:"title"`
	Language             string `json:"language"`
	Theme                string `json:"theme"`
	RefreshIntervalSec   int    `json:"refreshIntervalSec"`
	RequireLogin         bool   `json:"requireLogin"`
	AllowRegister        bool   `json:"allowRegister"`
	EnableTwoFactorLogin bool   `json:"enableTwoFactorLogin"`
	SubscriptionToken    string `json:"subscriptionToken"`
	PublicBaseURL        string `json:"publicBaseUrl"`
	AdminUsername        string `json:"adminUsername"`
	AdminPasswordHash    string `json:"adminPasswordHash"`
}

type AuditLog struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"createdAt"`
}

var ErrNotFound = errors.New("record not found")

func OpenDefaultStore() (*Store, error) {
	dbPath := os.Getenv("ZUI_DB")
	if dbPath == "" {
		dbPath = "./zui.db"
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS inbounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag TEXT NOT NULL DEFAULT '',
    remark TEXT NOT NULL,
    protocol TEXT NOT NULL,
    listen TEXT NOT NULL DEFAULT '0.0.0.0',
    port INTEGER NOT NULL UNIQUE,
    total_gb INTEGER NOT NULL DEFAULT 0,
    used_gb INTEGER NOT NULL DEFAULT 0,
    device_limit INTEGER NOT NULL DEFAULT 0,
    expiry_at TEXT NOT NULL DEFAULT '',
    authentication TEXT NOT NULL DEFAULT '',
    decryption TEXT NOT NULL DEFAULT 'none',
    encryption TEXT NOT NULL DEFAULT '',
    transport TEXT NOT NULL DEFAULT 'tcp',
    security TEXT NOT NULL DEFAULT 'none',
    proxy_protocol INTEGER NOT NULL DEFAULT 0,
    settings_json TEXT NOT NULL DEFAULT '{}',
    stream_settings_json TEXT NOT NULL DEFAULT '{}',
    sniffing_json TEXT NOT NULL DEFAULT '{}',
    fallbacks_json TEXT NOT NULL DEFAULT '[]',
    sockopt_json TEXT NOT NULL DEFAULT '{}',
    http_obfs_json TEXT NOT NULL DEFAULT '{}',
    external_proxy_json TEXT NOT NULL DEFAULT '{}',
    enable INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("migrate inbounds: %w", err)
	}

	const auditDDL = `
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,
    target TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    ip TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`
	if _, err := s.db.Exec(auditDDL); err != nil {
		return fmt.Errorf("migrate audit_logs: %w", err)
	}

	if err := ensureColumn(s.db, "inbounds", "tag", "ALTER TABLE inbounds ADD COLUMN tag TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "listen", "ALTER TABLE inbounds ADD COLUMN listen TEXT NOT NULL DEFAULT '0.0.0.0'"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "total_gb", "ALTER TABLE inbounds ADD COLUMN total_gb INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "used_gb", "ALTER TABLE inbounds ADD COLUMN used_gb INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "device_limit", "ALTER TABLE inbounds ADD COLUMN device_limit INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "expiry_at", "ALTER TABLE inbounds ADD COLUMN expiry_at TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "authentication", "ALTER TABLE inbounds ADD COLUMN authentication TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "decryption", "ALTER TABLE inbounds ADD COLUMN decryption TEXT NOT NULL DEFAULT 'none'"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "encryption", "ALTER TABLE inbounds ADD COLUMN encryption TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "transport", "ALTER TABLE inbounds ADD COLUMN transport TEXT NOT NULL DEFAULT 'tcp'"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "security", "ALTER TABLE inbounds ADD COLUMN security TEXT NOT NULL DEFAULT 'none'"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "proxy_protocol", "ALTER TABLE inbounds ADD COLUMN proxy_protocol INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "fallbacks_json", "ALTER TABLE inbounds ADD COLUMN fallbacks_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "sockopt_json", "ALTER TABLE inbounds ADD COLUMN sockopt_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "http_obfs_json", "ALTER TABLE inbounds ADD COLUMN http_obfs_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := ensureColumn(s.db, "inbounds", "external_proxy_json", "ALTER TABLE inbounds ADD COLUMN external_proxy_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}

	const settingsDDL = `
CREATE TABLE IF NOT EXISTS panel_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    title TEXT NOT NULL DEFAULT 'Z-UI 管理面板',
    language TEXT NOT NULL DEFAULT 'zh-CN',
    theme TEXT NOT NULL DEFAULT 'default',
    refresh_interval_sec INTEGER NOT NULL DEFAULT 30,
    require_login INTEGER NOT NULL DEFAULT 1,
    allow_register INTEGER NOT NULL DEFAULT 0,
    enable_two_factor_login INTEGER NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`
	if _, err := s.db.Exec(settingsDDL); err != nil {
		return fmt.Errorf("migrate panel_settings: %w", err)
	}
	if err := ensureColumn(s.db, "panel_settings", "subscription_token", "ALTER TABLE panel_settings ADD COLUMN subscription_token TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate panel_settings.subscription_token: %w", err)
	}
	if err := ensureColumn(s.db, "panel_settings", "public_base_url", "ALTER TABLE panel_settings ADD COLUMN public_base_url TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate panel_settings.public_base_url: %w", err)
	}
	if err := ensureColumn(s.db, "panel_settings", "admin_username", "ALTER TABLE panel_settings ADD COLUMN admin_username TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate panel_settings.admin_username: %w", err)
	}
	if err := ensureColumn(s.db, "panel_settings", "admin_password_hash", "ALTER TABLE panel_settings ADD COLUMN admin_password_hash TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate panel_settings.admin_password_hash: %w", err)
	}
	if _, err := s.db.Exec(`
INSERT INTO panel_settings (id)
SELECT 1
WHERE NOT EXISTS (SELECT 1 FROM panel_settings WHERE id = 1)`); err != nil {
		return fmt.Errorf("init panel_settings row: %w", err)
	}
	return nil
}

func (s *Store) ListInbounds() ([]Inbound, error) {
	rows, err := s.db.Query(inboundSelectSQL + ` ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Inbound
	for rows.Next() {
		item, err := scanInbound(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) CountInbounds() (total int, enabled int, err error) {
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM inbounds`).Scan(&total); err != nil {
		return 0, 0, err
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM inbounds WHERE enable = 1`).Scan(&enabled); err != nil {
		return 0, 0, err
	}
	return total, enabled, nil
}

func (s *Store) SumInboundTrafficGB() (totalGB int64, usedGB int64, err error) {
	if err = s.db.QueryRow(`SELECT COALESCE(SUM(total_gb), 0), COALESCE(SUM(used_gb), 0) FROM inbounds`).Scan(&totalGB, &usedGB); err != nil {
		return 0, 0, err
	}
	return totalGB, usedGB, nil
}

func (s *Store) ListEnabledInbounds() ([]Inbound, error) {
	rows, err := s.db.Query(inboundSelectSQL + ` WHERE enable = 1 ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Inbound
	for rows.Next() {
		item, err := scanInbound(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) GetInbound(id int64) (Inbound, error) {
	row := s.db.QueryRow(inboundSelectSQL+` WHERE id = ?`, id)
	item, err := scanInbound(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Inbound{}, ErrNotFound
	}
	return item, err
}

func (s *Store) CreateInbound(in Inbound) (Inbound, error) {
	res, err := s.db.Exec(`
INSERT INTO inbounds (tag, remark, protocol, listen, port, total_gb, used_gb, device_limit, expiry_at, authentication, decryption, encryption, transport, security, proxy_protocol, settings_json, stream_settings_json, sniffing_json, fallbacks_json, sockopt_json, http_obfs_json, external_proxy_json, enable)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Tag,
		in.Remark,
		in.Protocol,
		in.Listen,
		in.Port,
		in.TotalGB,
		in.UsedGB,
		in.DeviceLimit,
		timeToDBString(in.ExpiryAt),
		in.Authentication,
		in.Decryption,
		in.Encryption,
		in.Transport,
		in.Security,
		boolToInt(in.ProxyProtocol),
		in.SettingsJSON,
		in.StreamSettingsJSON,
		in.SniffingJSON,
		in.FallbacksJSON,
		in.SockoptJSON,
		in.HTTPObfsJSON,
		in.ExternalProxyJSON,
		boolToInt(in.Enable),
	)
	if err != nil {
		return Inbound{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Inbound{}, err
	}
	return s.GetInbound(id)
}

func (s *Store) UpdateInbound(id int64, in Inbound) (Inbound, error) {
	res, err := s.db.Exec(`
UPDATE inbounds
SET tag = ?, remark = ?, protocol = ?, listen = ?, port = ?, total_gb = ?, used_gb = ?, device_limit = ?, expiry_at = ?, authentication = ?, decryption = ?, encryption = ?, transport = ?, security = ?, proxy_protocol = ?, settings_json = ?, stream_settings_json = ?, sniffing_json = ?, fallbacks_json = ?, sockopt_json = ?, http_obfs_json = ?, external_proxy_json = ?, enable = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`,
		in.Tag,
		in.Remark,
		in.Protocol,
		in.Listen,
		in.Port,
		in.TotalGB,
		in.UsedGB,
		in.DeviceLimit,
		timeToDBString(in.ExpiryAt),
		in.Authentication,
		in.Decryption,
		in.Encryption,
		in.Transport,
		in.Security,
		boolToInt(in.ProxyProtocol),
		in.SettingsJSON,
		in.StreamSettingsJSON,
		in.SniffingJSON,
		in.FallbacksJSON,
		in.SockoptJSON,
		in.HTTPObfsJSON,
		in.ExternalProxyJSON,
		boolToInt(in.Enable),
		id,
	)
	if err != nil {
		return Inbound{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Inbound{}, err
	}
	if affected == 0 {
		return Inbound{}, ErrNotFound
	}
	return s.GetInbound(id)
}

func (s *Store) DeleteInbound(id int64) error {
	res, err := s.db.Exec(`DELETE FROM inbounds WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateInboundUsedGBByTag(tag string, usedGB int64) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil
	}
	_, err := s.db.Exec(`
UPDATE inbounds
SET used_gb = ?, updated_at = CURRENT_TIMESTAMP
WHERE tag = ?`, usedGB, tag)
	return err
}

const inboundSelectSQL = `
SELECT id, tag, remark, protocol, listen, port, total_gb, used_gb, device_limit, expiry_at, authentication, decryption, encryption, transport, security, proxy_protocol, settings_json, stream_settings_json, sniffing_json, fallbacks_json, sockopt_json, http_obfs_json, external_proxy_json, enable, created_at, updated_at
FROM inbounds`

type scanner interface {
	Scan(dest ...any) error
}

func scanInbound(s scanner) (Inbound, error) {
	var item Inbound
	var proxyProtocolInt int
	var enableInt int
	var expiryAtRaw string
	err := s.Scan(
		&item.ID,
		&item.Tag,
		&item.Remark,
		&item.Protocol,
		&item.Listen,
		&item.Port,
		&item.TotalGB,
		&item.UsedGB,
		&item.DeviceLimit,
		&expiryAtRaw,
		&item.Authentication,
		&item.Decryption,
		&item.Encryption,
		&item.Transport,
		&item.Security,
		&proxyProtocolInt,
		&item.SettingsJSON,
		&item.StreamSettingsJSON,
		&item.SniffingJSON,
		&item.FallbacksJSON,
		&item.SockoptJSON,
		&item.HTTPObfsJSON,
		&item.ExternalProxyJSON,
		&enableInt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Inbound{}, err
	}
	item.ProxyProtocol = proxyProtocolInt == 1
	item.Enable = enableInt == 1
	if t, ok := parseDBTime(expiryAtRaw); ok {
		item.ExpiryAt = &t
	}
	return item, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func intToBool(v int) bool {
	return v == 1
}

func parseDBTime(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func timeToDBString(v *time.Time) string {
	if v == nil || v.IsZero() {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}

func (s *Store) GetPanelSettings() (PanelSettings, error) {
	var item PanelSettings
	var requireLogin, allowRegister, enableTwoFactor int
	err := s.db.QueryRow(`
SELECT title, language, theme, refresh_interval_sec, require_login, allow_register, enable_two_factor_login
FROM panel_settings
WHERE id = 1`).Scan(
		&item.Title,
		&item.Language,
		&item.Theme,
		&item.RefreshIntervalSec,
		&requireLogin,
		&allowRegister,
		&enableTwoFactor,
	)
	if err != nil {
		return PanelSettings{}, err
	}
	err = s.db.QueryRow(`
SELECT subscription_token, public_base_url, admin_username, admin_password_hash
FROM panel_settings
WHERE id = 1`).Scan(
		&item.SubscriptionToken,
		&item.PublicBaseURL,
		&item.AdminUsername,
		&item.AdminPasswordHash,
	)
	if err != nil {
		return PanelSettings{}, err
	}
	item.RequireLogin = intToBool(requireLogin)
	item.AllowRegister = intToBool(allowRegister)
	item.EnableTwoFactorLogin = intToBool(enableTwoFactor)
	return item, nil
}

func (s *Store) UpdatePanelSettings(in PanelSettings) (PanelSettings, error) {
	_, err := s.db.Exec(`
UPDATE panel_settings
SET title = ?, language = ?, theme = ?, refresh_interval_sec = ?, require_login = ?, allow_register = ?, enable_two_factor_login = ?, subscription_token = ?, public_base_url = ?, admin_username = ?, admin_password_hash = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = 1`,
		in.Title,
		in.Language,
		in.Theme,
		in.RefreshIntervalSec,
		boolToInt(in.RequireLogin),
		boolToInt(in.AllowRegister),
		boolToInt(in.EnableTwoFactorLogin),
		in.SubscriptionToken,
		in.PublicBaseURL,
		in.AdminUsername,
		in.AdminPasswordHash,
	)
	if err != nil {
		return PanelSettings{}, err
	}
	return s.GetPanelSettings()
}

func (s *Store) AddAuditLog(action, target, detail, username, ip string) error {
	action = strings.TrimSpace(action)
	if action == "" {
		action = "unknown"
	}
	_, err := s.db.Exec(`
INSERT INTO audit_logs (action, target, detail, username, ip)
VALUES (?, ?, ?, ?, ?)`,
		action,
		strings.TrimSpace(target),
		strings.TrimSpace(detail),
		strings.TrimSpace(username),
		strings.TrimSpace(ip),
	)
	return err
}

func (s *Store) ListAuditLogs(limit, offset int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.Query(`
SELECT id, action, target, detail, username, ip, created_at
FROM audit_logs
ORDER BY id DESC
LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AuditLog, 0, limit)
	for rows.Next() {
		var item AuditLog
		var createdRaw string
		if err := rows.Scan(&item.ID, &item.Action, &item.Target, &item.Detail, &item.Username, &item.IP, &createdRaw); err != nil {
			return nil, err
		}
		if t, ok := parseDBTime(createdRaw); ok {
			item.CreatedAt = t
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func ensureColumn(db *sql.DB, table, column, alterDDL string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(alterDDL)
	return err
}

func (s *Store) SetAdminCredentials(username, passwordHash string) error {
	_, err := s.db.Exec(`
UPDATE panel_settings
SET admin_username = ?, admin_password_hash = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = 1`, username, passwordHash)
	return err
}

func (s *Store) GetAdminCredentials() (username string, passwordHash string, err error) {
	err = s.db.QueryRow(`
SELECT admin_username, admin_password_hash
FROM panel_settings
WHERE id = 1`).Scan(&username, &passwordHash)
	return
}
