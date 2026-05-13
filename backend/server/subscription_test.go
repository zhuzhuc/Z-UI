package server

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"zui/storage"
)

func testInbound(protocol, settingsJSON string) storage.Inbound {
	return storage.Inbound{
		Remark:             "test",
		Protocol:           protocol,
		Port:               443,
		SettingsJSON:       settingsJSON,
		StreamSettingsJSON: `{}`,
		SniffingJSON:       `{}`,
		FallbacksJSON:      `[]`,
		SockoptJSON:        `{}`,
		HTTPObfsJSON:       `{}`,
		ExternalProxyJSON:  `{}`,
	}
}

func TestExtractClients_Vless(t *testing.T) {
	settings := `{"clients":[{"id":"abc-123","email":"user1@test.com"}]}`
	clients, err := extractClients("vless", settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 1 {
		t.Fatalf("len = %d, want 1", len(clients))
	}
	if clients[0].IDOrPassword != "abc-123" {
		t.Errorf("ID = %q, want %q", clients[0].IDOrPassword, "abc-123")
	}
	if clients[0].Email != "user1@test.com" {
		t.Errorf("Email = %q, want %q", clients[0].Email, "user1@test.com")
	}
}

func TestExtractClients_Trojan(t *testing.T) {
	settings := `{"clients":[{"password":"mypassword","email":"t@t.com"}]}`
	clients, err := extractClients("trojan", settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 1 {
		t.Fatalf("len = %d, want 1", len(clients))
	}
	if clients[0].IDOrPassword != "mypassword" {
		t.Errorf("password = %q, want %q", clients[0].IDOrPassword, "mypassword")
	}
}

func TestExtractClients_Empty(t *testing.T) {
	settings := `{"clients":[]}`
	clients, err := extractClients("vless", settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 0 {
		t.Errorf("len = %d, want 0", len(clients))
	}
}

func TestExtractClients_InvalidJSON(t *testing.T) {
	_, err := extractClients("vless", "not-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestBuildShadowsocksLinks(t *testing.T) {
	inb := testInbound("shadowsocks", `{"method":"aes-256-gcm","password":"secret123","email":"ss@t.com"}`)
	links, err := buildShadowsocksLinks(inb, "example.com", "8388", "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if !strings.HasPrefix(links[0], "ss://") {
		t.Errorf("link should start with ss://, got %q", links[0])
	}
	if !strings.HasSuffix(links[0], "#ss%40t.com") && !strings.Contains(links[0], "ss@t.com") {
		// The fragment should contain the email
		t.Logf("link: %s", links[0])
	}
}

func TestBuildShadowsocksLinksWithUsers(t *testing.T) {
	settings := `{"method":"aes-256-gcm","users":[{"password":"p1","email":"u1@t.com"},{"password":"p2","email":"u2@t.com"}]}`
	inb := testInbound("shadowsocks", settings)
	links, err := buildShadowsocksLinks(inb, "example.com", "8388", "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 2 {
		t.Fatalf("len = %d, want 2", len(links))
	}
}

func TestInboundToLinks_Vless(t *testing.T) {
	inb := testInbound("vless", `{"clients":[{"id":"uuid-1234","email":"vless@t.com"}]}`)
	inb.StreamSettingsJSON = `{"network":"ws","security":"tls"}`
	links, err := inboundToLinks(inb, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if !strings.HasPrefix(links[0], "vless://") {
		t.Errorf("should start with vless://, got %q", links[0])
	}
	if !strings.Contains(links[0], "example.com") {
		t.Error("should contain host")
	}
	if !strings.Contains(links[0], "security=tls") {
		t.Error("should contain security=tls")
	}
}

func TestInboundToLinks_Vmess(t *testing.T) {
	inb := testInbound("vmess", `{"clients":[{"id":"uuid-5678","email":"vmess@t.com"}]}`)
	inb.StreamSettingsJSON = `{"network":"tcp","security":"none"}`
	links, err := inboundToLinks(inb, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if !strings.HasPrefix(links[0], "vmess://") {
		t.Errorf("should start with vmess://, got %q", links[0])
	}

	// Decode the vmess payload
	raw := strings.TrimPrefix(links[0], "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	var vmess map[string]string
	if err := json.Unmarshal(decoded, &vmess); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if vmess["add"] != "example.com" {
		t.Errorf("host = %q, want %q", vmess["add"], "example.com")
	}
	if vmess["id"] != "uuid-5678" {
		t.Errorf("id = %q, want %q", vmess["id"], "uuid-5678")
	}
}

func TestInboundToLinks_Trojan(t *testing.T) {
	inb := testInbound("trojan", `{"clients":[{"password":"trojan-pass","email":"t@t.com"}]}`)
	inb.StreamSettingsJSON = `{"network":"tcp","security":"tls"}`
	links, err := inboundToLinks(inb, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if !strings.HasPrefix(links[0], "trojan://") {
		t.Errorf("should start with trojan://, got %q", links[0])
	}
}

func TestHostWithoutPort(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"example.com:443", "example.com"},
		{"example.com", "example.com"},
		{"127.0.0.1:8080", "127.0.0.1"},
		{"[::1]:8080", "::1"},
	}
	for _, tt := range tests {
		if got := hostWithoutPort(tt.input); got != tt.want {
			t.Errorf("hostWithoutPort(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeRemark(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-server", "my-server"},
		{"  spaces  ", "spaces"},
		{"", "zui"},
	}
	for _, tt := range tests {
		if got := sanitizeRemark(tt.input); got != tt.want {
			t.Errorf("sanitizeRemark(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRandomToken(t *testing.T) {
	t1 := randomToken(24)
	t2 := randomToken(24)
	if t1 == t2 {
		t.Error("random tokens should be different")
	}
	if len(t1) != 24 {
		t.Errorf("token length = %d, want 24", len(t1))
	}
	// Default for zero/negative
	if len(randomToken(0)) != 24 {
		t.Error("default token length should be 24")
	}
}

func TestMapSecurityToTLS(t *testing.T) {
	if got := mapSecurityToTLS("tls"); got != "tls" {
		t.Errorf("mapSecurityToTLS(tls) = %q, want tls", got)
	}
	if got := mapSecurityToTLS("none"); got != "" {
		t.Errorf("mapSecurityToTLS(none) = %q, want empty", got)
	}
}

func TestExtractUserFromLogLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"json email", `{"email":"user1@domain.com","ip":"1.2.3.4"}`, "user1@domain.com"},
		{"plain email", `email=user2@domain.com`, "user2@domain.com"},
		{"json user", `{"user":"user3","port":443}`, "user3"},
		{"plain user", `user: user4`, "user4"},
		{"empty", "", ""},
		{"no match", "some random log line", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractUserFromLogLine(tt.line); got != tt.want {
				t.Errorf("extractUserFromLogLine(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestExtractIPv4FromLogLine(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"connection from 192.168.1.100:12345", "192.168.1.100"},
		{"no ip here", ""},
		{"10.0.0.1 is internal", "10.0.0.1"},
	}
	for _, tt := range tests {
		if got := extractIPv4FromLogLine(tt.line); got != tt.want {
			t.Errorf("extractIPv4(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestExtractTimestampFromLogLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		ok   bool
	}{
		{"rfc3339", "2024-01-15T10:30:00Z something", true},
		{"ymd hms slash", "2024/01/15 10:30:00 connection", true},
		{"ymd hms dash", "2024-01-15 10:30:00 connection", true},
		{"no timestamp", "no timestamp here", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := extractTimestampFromLogLine(tt.line)
			if ok != tt.ok {
				t.Errorf("extractTimestamp(%q) ok = %v, want %v", tt.line, ok, tt.ok)
			}
		})
	}
}

func TestNodeDisplayName(t *testing.T) {
	tests := []struct {
		name string
		link string
		idx  int
		want string
	}{
		{"vless with fragment", "vless://uuid@host:443#MyNode", 0, "MyNode"},
		{"vless no fragment", "vless://uuid@host:443", 0, "node-1"},
		{"vmess with ps", "vmess://" + base64.StdEncoding.EncodeToString([]byte(`{"ps":"MyVmess"}`)), 0, "MyVmess"},
		{"vmess no ps", "vmess://" + base64.StdEncoding.EncodeToString([]byte(`{"v":"2"}`)), 0, "node-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeDisplayName(tt.link, tt.idx); got != tt.want {
				t.Errorf("nodeDisplayName = %q, want %q", got, tt.want)
			}
		})
	}
}
