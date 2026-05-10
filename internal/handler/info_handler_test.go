package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGetInfo(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest(http.MethodGet, "/api/info", nil)
	w := httptest.NewRecorder()

	h.handleGetInfo(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool             `json:"success"`
		Data    *UserSystemInfo  `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data == nil {
		t.Fatal("expected data to be non-nil")
	}

	info := resp.Data

	// Version should be set.
	if info.Version == "" {
		t.Error("expected version to be set")
	}

	// Config checks.
	if info.Config == nil {
		t.Fatal("expected config to be non-nil")
	}
	if info.Config.ConfigVersion != "v8" {
		t.Errorf("expected config_version=v8, got %s", info.Config.ConfigVersion)
	}
	if info.Config.ExecutorProfile == nil || info.Config.ExecutorProfile.Executor != "CLAUDE_CODE" {
		t.Error("expected executor_profile.executor=CLAUDE_CODE")
	}

	// Login status should be logged out.
	ls, ok := info.LoginStatus.(map[string]interface{})
	if !ok {
		t.Fatalf("expected login_status to be a map, got %T", info.LoginStatus)
	}
	if ls["status"] != "loggedout" {
		t.Errorf("expected login_status.status=loggedout, got %v", ls["status"])
	}

	// Environment checks.
	if info.Environment == nil {
		t.Fatal("expected environment to be non-nil")
	}
	if info.Environment.OSType == "" {
		t.Error("expected os_type to be set")
	}
	if info.Environment.Bitness != "64" && info.Environment.Bitness != "32" {
		t.Errorf("expected bitness to be 32 or 64, got %s", info.Environment.Bitness)
	}

	// Executors checks.
	if len(info.Executors) == 0 {
		t.Error("expected at least one executor")
	}
	cc, ok := info.Executors["CLAUDE_CODE"]
	if !ok {
		t.Error("expected CLAUDE_CODE executor")
	}
	if !cc.Available {
		t.Error("expected CLAUDE_CODE to be available")
	}

	// Capabilities checks.
	caps, ok := info.Capabilities["CLAUDE_CODE"]
	if !ok {
		t.Error("expected CLAUDE_CODE capabilities")
	}
	found := false
	for _, c := range caps {
		if c == "SESSION_FORK" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SESSION_FORK capability for CLAUDE_CODE")
	}

	// Machine ID should be set.
	if info.MachineID == "" {
		t.Error("expected machine_id to be set")
	}
}

func TestBuildDefaultConfig(t *testing.T) {
	cfg := buildDefaultConfig()

	if cfg.ConfigVersion != "v8" {
		t.Errorf("expected v8, got %s", cfg.ConfigVersion)
	}
	if cfg.Theme != "System" {
		t.Errorf("expected System theme, got %s", cfg.Theme)
	}
	if cfg.GitBranchPrefix != "vk" {
		t.Errorf("expected vk prefix, got %s", cfg.GitBranchPrefix)
	}
	if cfg.SendMessageShortcut != "ModifierEnter" {
		t.Errorf("expected ModifierEnter, got %s", cfg.SendMessageShortcut)
	}
	if !cfg.PRAutoDescriptionEnabled {
		t.Error("expected pr_auto_description_enabled=true")
	}
	if !cfg.CommitReminderEnabled {
		t.Error("expected commit_reminder_enabled=true")
	}
	if cfg.Notifications == nil {
		t.Error("expected notifications to be set")
	}
	if cfg.Editor == nil {
		t.Error("expected editor to be set")
	}
	if cfg.GitHub == nil {
		t.Error("expected github to be set")
	}
	if cfg.Showcases == nil {
		t.Error("expected showcases to be set")
	}
}

func TestDetectEnvironment(t *testing.T) {
	env := detectEnvironment()
	if env == nil {
		t.Fatal("expected environment to be non-nil")
	}
	if env.OSType == "" {
		t.Error("expected os_type to be set")
	}
	if env.OSArchitecture == "" {
		t.Error("expected os_architecture to be set")
	}
	if env.Bitness != "32" && env.Bitness != "64" {
		t.Errorf("expected bitness 32 or 64, got %s", env.Bitness)
	}
}
