package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	os.Setenv("ZUI_DB", dbPath)
	t.Cleanup(func() { os.Unsetenv("ZUI_DB") })

	store, err := OpenDefaultStore()
	if err != nil {
		t.Fatalf("OpenDefaultStore failed: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestMigration(t *testing.T) {
	store := openTestStore(t)
	// Verify tables exist by querying them
	_, err := store.ListInbounds()
	if err != nil {
		t.Fatalf("ListInbounds after migration failed: %v", err)
	}
	_, err = store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers after migration failed: %v", err)
	}
}

func TestInboundCRUD(t *testing.T) {
	store := openTestStore(t)

	// Create
	in := Inbound{
		Remark:             "test-inbound",
		Protocol:           "vless",
		Listen:             "0.0.0.0",
		Port:               443,
		SettingsJSON:       `{"clients":[]}`,
		StreamSettingsJSON: `{}`,
		SniffingJSON:       `{}`,
		FallbacksJSON:      `[]`,
		SockoptJSON:        `{}`,
		HTTPObfsJSON:       `{}`,
		ExternalProxyJSON:  `{}`,
		Enable:             true,
	}
	created, err := store.CreateInbound(in)
	if err != nil {
		t.Fatalf("CreateInbound failed: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("created inbound ID should not be 0")
	}
	if created.Remark != "test-inbound" {
		t.Errorf("Remark = %q, want %q", created.Remark, "test-inbound")
	}
	if created.Port != 443 {
		t.Errorf("Port = %d, want 443", created.Port)
	}

	// Read
	got, err := store.GetInbound(created.ID)
	if err != nil {
		t.Fatalf("GetInbound failed: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}

	// Update
	in.Remark = "updated-inbound"
	in.Port = 8443
	updated, err := store.UpdateInbound(created.ID, in)
	if err != nil {
		t.Fatalf("UpdateInbound failed: %v", err)
	}
	if updated.Remark != "updated-inbound" {
		t.Errorf("Remark = %q, want %q", updated.Remark, "updated-inbound")
	}

	// List
	list, err := store.ListInbounds()
	if err != nil {
		t.Fatalf("ListInbounds failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListInbounds len = %d, want 1", len(list))
	}

	// Delete
	if err := store.DeleteInbound(created.ID); err != nil {
		t.Fatalf("DeleteInbound failed: %v", err)
	}
	_, err = store.GetInbound(created.ID)
	if err != ErrNotFound {
		t.Errorf("GetInbound after delete err = %v, want ErrNotFound", err)
	}
}

func TestInboundDuplicatePort(t *testing.T) {
	store := openTestStore(t)

	in1 := Inbound{Remark: "a", Protocol: "vless", Port: 443, SettingsJSON: "{}", StreamSettingsJSON: "{}", SniffingJSON: "{}", FallbacksJSON: "[]", SockoptJSON: "{}", HTTPObfsJSON: "{}", ExternalProxyJSON: "{}"}
	_, err := store.CreateInbound(in1)
	if err != nil {
		t.Fatalf("first CreateInbound failed: %v", err)
	}

	in2 := Inbound{Remark: "b", Protocol: "vless", Port: 443, SettingsJSON: "{}", StreamSettingsJSON: "{}", SniffingJSON: "{}", FallbacksJSON: "[]", SockoptJSON: "{}", HTTPObfsJSON: "{}", ExternalProxyJSON: "{}"}
	_, err = store.CreateInbound(in2)
	if err == nil {
		t.Fatal("expected error for duplicate port")
	}
}

func TestDeleteInboundNotFound(t *testing.T) {
	store := openTestStore(t)
	err := store.DeleteInbound(9999)
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUserCRUD(t *testing.T) {
	store := openTestStore(t)

	// Create
	user, err := store.CreateUser("testuser", "hashed_password", "admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("user ID should not be 0")
	}
	if user.Username != "testuser" {
		t.Errorf("Username = %q, want %q", user.Username, "testuser")
	}
	if user.Role != "admin" {
		t.Errorf("Role = %q, want %q", user.Role, "admin")
	}
	if user.Status != "active" {
		t.Errorf("Status = %q, want %q", user.Status, "active")
	}

	// Get by username
	got, err := store.GetUserByUsername("testuser")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("ID = %d, want %d", got.ID, user.ID)
	}

	// Get by ID
	got2, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got2.Username != "testuser" {
		t.Errorf("Username = %q, want %q", got2.Username, "testuser")
	}

	// Update role
	updated, err := store.UpdateUserRole(user.ID, "operator")
	if err != nil {
		t.Fatalf("UpdateUserRole failed: %v", err)
	}
	if updated.Role != "operator" {
		t.Errorf("Role = %q, want %q", updated.Role, "operator")
	}

	// Update username
	updated, err = store.UpdateUserUsername(user.ID, "newname")
	if err != nil {
		t.Fatalf("UpdateUserUsername failed: %v", err)
	}
	if updated.Username != "newname" {
		t.Errorf("Username = %q, want %q", updated.Username, "newname")
	}

	// Update password
	if err := store.UpdateUserPassword(user.ID, "new_hash"); err != nil {
		t.Fatalf("UpdateUserPassword failed: %v", err)
	}

	// List
	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("ListUsers len = %d, want 1", len(users))
	}

	// Delete
	if err := store.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
	_, err = store.GetUserByID(user.ID)
	if err != ErrNotFound {
		t.Errorf("GetUserByID after delete err = %v, want ErrNotFound", err)
	}
}

func TestDuplicateUsername(t *testing.T) {
	store := openTestStore(t)

	_, err := store.CreateUser("dup", "hash1", "admin")
	if err != nil {
		t.Fatalf("first CreateUser failed: %v", err)
	}
	_, err = store.CreateUser("dup", "hash2", "admin")
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestGetUserNotFound(t *testing.T) {
	store := openTestStore(t)

	_, err := store.GetUserByUsername("nonexistent")
	if err != ErrNotFound {
		t.Errorf("GetUserByUsername err = %v, want ErrNotFound", err)
	}
	_, err = store.GetUserByID(9999)
	if err != ErrNotFound {
		t.Errorf("GetUserByID err = %v, want ErrNotFound", err)
	}
}

func TestEnsureOwnerUser(t *testing.T) {
	store := openTestStore(t)

	// First call creates owner
	user, err := store.EnsureOwnerUser("admin", "hash1")
	if err != nil {
		t.Fatalf("EnsureOwnerUser failed: %v", err)
	}
	if user.Role != "owner" {
		t.Errorf("Role = %q, want %q", user.Role, "owner")
	}

	// Second call updates existing owner
	user2, err := store.EnsureOwnerUser("newadmin", "hash2")
	if err != nil {
		t.Fatalf("EnsureOwnerUser second call failed: %v", err)
	}
	if user2.ID != user.ID {
		t.Errorf("should update same user, got different ID")
	}
	if user2.Username != "newadmin" {
		t.Errorf("Username = %q, want %q", user2.Username, "newadmin")
	}
}

func TestCountUsersByRole(t *testing.T) {
	store := openTestStore(t)

	_, _ = store.CreateUser("admin1", "h", "admin")
	_, _ = store.CreateUser("admin2", "h", "admin")
	_, _ = store.CreateUser("viewer1", "h", "viewer")

	count, err := store.CountUsersByRole("admin")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("admin count = %d, want 2", count)
	}

	count, err = store.CountUsersByRole("viewer")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("viewer count = %d, want 1", count)
	}

	count, err = store.CountUsersByRole("owner")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("owner count = %d, want 0", count)
	}
}

func TestPanelSettings(t *testing.T) {
	store := openTestStore(t)

	settings, err := store.GetPanelSettings()
	if err != nil {
		t.Fatalf("GetPanelSettings failed: %v", err)
	}
	if settings.Title == "" {
		t.Error("default title should not be empty")
	}

	settings.Title = "Custom Title"
	settings.SubscriptionToken = "test-token"
	settings.PublicBaseURL = "https://example.com"
	updated, err := store.UpdatePanelSettings(settings)
	if err != nil {
		t.Fatalf("UpdatePanelSettings failed: %v", err)
	}
	if updated.Title != "Custom Title" {
		t.Errorf("Title = %q, want %q", updated.Title, "Custom Title")
	}
	if updated.SubscriptionToken != "test-token" {
		t.Errorf("SubscriptionToken = %q, want %q", updated.SubscriptionToken, "test-token")
	}
}

func TestAuditLog(t *testing.T) {
	store := openTestStore(t)

	if err := store.AddAuditLog("test.action", "target1", "detail1", "admin", "127.0.0.1"); err != nil {
		t.Fatalf("AddAuditLog failed: %v", err)
	}
	if err := store.AddAuditLog("test.action2", "target2", "detail2", "admin", "127.0.0.1"); err != nil {
		t.Fatalf("AddAuditLog failed: %v", err)
	}

	logs, err := store.ListAuditLogs(10, 0)
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("ListAuditLogs len = %d, want 2", len(logs))
	}
	// Most recent first
	if logs[0].Action != "test.action2" {
		t.Errorf("first log action = %q, want %q", logs[0].Action, "test.action2")
	}
}

func TestInboundTrafficAndCount(t *testing.T) {
	store := openTestStore(t)

	_, _ = store.CreateInbound(Inbound{Remark: "a", Protocol: "vless", Port: 1, TotalGB: 100, UsedGB: 10, SettingsJSON: "{}", StreamSettingsJSON: "{}", SniffingJSON: "{}", FallbacksJSON: "[]", SockoptJSON: "{}", HTTPObfsJSON: "{}", ExternalProxyJSON: "{}", Enable: true})
	_, _ = store.CreateInbound(Inbound{Remark: "b", Protocol: "vless", Port: 2, TotalGB: 200, UsedGB: 20, SettingsJSON: "{}", StreamSettingsJSON: "{}", SniffingJSON: "{}", FallbacksJSON: "[]", SockoptJSON: "{}", HTTPObfsJSON: "{}", ExternalProxyJSON: "{}", Enable: false})

	total, enabled, err := store.CountInbounds()
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if enabled != 1 {
		t.Errorf("enabled = %d, want 1", enabled)
	}

	totalGB, usedGB, err := store.SumInboundTrafficGB()
	if err != nil {
		t.Fatal(err)
	}
	if totalGB != 300 {
		t.Errorf("totalGB = %d, want 300", totalGB)
	}
	if usedGB != 30 {
		t.Errorf("usedGB = %d, want 30", usedGB)
	}
}
