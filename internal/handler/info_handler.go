package handler

import (
	"net/http"
	"runtime"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// Version is set at build time via -ldflags.
var Version = "0.1.44"

// handleGetInfo handles GET /api/info.
// Returns system information including version, config, environment, and executor profiles.
func (h *Handler) handleGetInfo(w http.ResponseWriter, r *http.Request) {
	info := buildUserSystemInfo()
	success(w, info)
}

// buildUserSystemInfo constructs the full UserSystemInfo response.
func buildUserSystemInfo() *UserSystemInfo {
	defaultConfig := buildDefaultConfig()

	executors := map[string]*ExecutorProfile{
		string(domain.AgentClaudeCode): {
			Executor:        string(domain.AgentClaudeCode),
			Command:         "claude",
			DisplayName:     "Claude Code",
			Available:       true,
			SupportsResume:  true,
			SupportsReview:  true,
			SupportsSetup:   false,
			Capabilities:    []string{"SESSION_FORK"},
		},
	}

	return &UserSystemInfo{
		Version:             Version,
		Config:              defaultConfig,
		MachineID:           generateMachineID(),
		LoginStatus:         LoginStatusLoggedOut,
		RemoteAuthDegraded:  nil,
		Environment:         detectEnvironment(),
		Capabilities:        map[string][]string{string(domain.AgentClaudeCode): {"SESSION_FORK"}},
		SharedAPIBase:       nil,
		PreviewProxyPort:    nil,
		Executors:           executors,
	}
}

// UserSystemInfo matches the frontend TypeScript UserSystemInfo type.
type UserSystemInfo struct {
	Version            string                        `json:"version"`
	Config             *Config                       `json:"config"`
	MachineID          string                        `json:"machine_id"`
	LoginStatus        LoginStatus                   `json:"login_status"`
	RemoteAuthDegraded *string                       `json:"remote_auth_degraded"`
	Environment        *Environment                  `json:"environment"`
	Capabilities       map[string][]string           `json:"capabilities"`
	SharedAPIBase      *string                       `json:"shared_api_base"`
	PreviewProxyPort   *int                          `json:"preview_proxy_port"`
	Executors          map[string]*ExecutorProfile   `json:"executors"`
}

// LoginStatus represents the user's login state.
// Serialized as either {"status":"loggedout"} or {"status":"loggedin","profile":{...}}.
type LoginStatus interface{}

var LoginStatusLoggedOut LoginStatus = map[string]string{"status": "loggedout"}

// Environment holds OS/platform information.
type Environment struct {
	OSType          string `json:"os_type"`
	OSVersion       string `json:"os_version"`
	OSArchitecture  string `json:"os_architecture"`
	Bitness         string `json:"bitness"`
}

// Config matches the frontend TypeScript Config type (v8).
type Config struct {
	ConfigVersion              string                `json:"config_version"`
	Theme                      string                `json:"theme"`
	ExecutorProfile            *ExecutorProfileID    `json:"executor_profile"`
	DisclaimerAcknowledged     bool                  `json:"disclaimer_acknowledged"`
	OnboardingAcknowledged     bool                  `json:"onboarding_acknowledged"`
	RemoteOnboardingAcknowledged bool                `json:"remote_onboarding_acknowledged"`
	Notifications              *NotificationConfig   `json:"notifications"`
	Editor                     *EditorConfig         `json:"editor"`
	GitHub                     *GitHubConfig         `json:"github"`
	AnalyticsEnabled           bool                  `json:"analytics_enabled"`
	WorkspaceDir               *string               `json:"workspace_dir"`
	LastAppVersion             *string               `json:"last_app_version"`
	ShowReleaseNotes           bool                  `json:"show_release_notes"`
	Language                   string                `json:"language"`
	GitBranchPrefix            string                `json:"git_branch_prefix"`
	Showcases                  *ShowcaseState        `json:"showcases"`
	PRAutoDescriptionEnabled   bool                  `json:"pr_auto_description_enabled"`
	PRAutoDescriptionPrompt    *string               `json:"pr_auto_description_prompt"`
	CommitReminderEnabled      bool                  `json:"commit_reminder_enabled"`
	CommitReminderPrompt       *string               `json:"commit_reminder_prompt"`
	SendMessageShortcut        string                `json:"send_message_shortcut"`
	RelayEnabled               bool                  `json:"relay_enabled"`
	HostNickname               *string               `json:"host_nickname"`
}

// ExecutorProfileID identifies the selected executor and optional variant.
type ExecutorProfileID struct {
	Executor string  `json:"executor"`
	Variant  *string `json:"variant"`
}

// NotificationConfig controls notification preferences.
type NotificationConfig struct {
	SoundEnabled bool   `json:"sound_enabled"`
	PushEnabled  bool   `json:"push_enabled"`
	SoundFile    string `json:"sound_file"`
}

// EditorConfig configures the user's code editor.
type EditorConfig struct {
	EditorType          string  `json:"editor_type"`
	CustomCommand       *string `json:"custom_command"`
	RemoteSSHHost       *string `json:"remote_ssh_host"`
	RemoteSSHUser       *string `json:"remote_ssh_user"`
	AutoInstallExtension bool   `json:"auto_install_extension"`
}

// GitHubConfig holds GitHub authentication state.
type GitHubConfig struct {
	PAT          *string `json:"pat"`
	OAuthToken   *string `json:"oauth_token"`
	Username     *string `json:"username"`
	PrimaryEmail *string `json:"primary_email"`
	DefaultPRBase *string `json:"default_pr_base"`
}

// ShowcaseState tracks which feature showcases the user has seen.
type ShowcaseState struct {
	SeenFeatures []string `json:"seen_features"`
}

// ExecutorProfile describes a single coding agent executor.
type ExecutorProfile struct {
	Executor       string   `json:"executor"`
	Command        string   `json:"command"`
	DisplayName    string   `json:"display_name"`
	Available      bool     `json:"available"`
	SupportsResume bool     `json:"supports_resume"`
	SupportsReview bool     `json:"supports_review"`
	SupportsSetup  bool     `json:"supports_setup"`
	Capabilities   []string `json:"capabilities"`
}

// buildDefaultConfig returns a sensible default Config for local mode.
func buildDefaultConfig() *Config {
	return &Config{
		ConfigVersion:              "v8",
		Theme:                      "System",
		ExecutorProfile:            &ExecutorProfileID{Executor: string(domain.AgentClaudeCode)},
		DisclaimerAcknowledged:     false,
		OnboardingAcknowledged:     false,
		RemoteOnboardingAcknowledged: false,
		Notifications: &NotificationConfig{
			SoundEnabled: true,
			PushEnabled:  true,
			SoundFile:    "ABSTRACT_SOUND1",
		},
		Editor: &EditorConfig{
			EditorType:          "VS_CODE",
			AutoInstallExtension: true,
		},
		GitHub:                   &GitHubConfig{},
		AnalyticsEnabled:         false,
		Language:                 "BROWSER",
		GitBranchPrefix:         "vk",
		Showcases:               &ShowcaseState{SeenFeatures: []string{}},
		PRAutoDescriptionEnabled: true,
		CommitReminderEnabled:   true,
		SendMessageShortcut:     "ModifierEnter",
		RelayEnabled:            true,
	}
}

// detectEnvironment returns OS/platform information.
func detectEnvironment() *Environment {
	osType := runtime.GOOS
	osArch := runtime.GOARCH
	bitness := "64"
	if osArch == "arm" || osArch == "386" {
		bitness = "32"
	}

	// Map Go OS names to user-friendly names.
	osName := osType
	switch osType {
	case "darwin":
		osName = "Macos"
	case "windows":
		osName = "Windows"
	case "linux":
		osName = "Linux"
	}

	// Map Go arch names to user-friendly names.
	archName := osArch
	switch osArch {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	case "arm":
		archName = "arm"
	case "386":
		archName = "x86"
	}

	return &Environment{
		OSType:         osName,
		OSVersion:      "unknown",
		OSArchitecture: archName,
		Bitness:        bitness,
	}
}

// generateMachineID returns a stable machine identifier.
// In local mode, this is a fixed placeholder. A production implementation
// would read from a persisted file or use a system-level identifier.
func generateMachineID() string {
	return "local-dev-machine"
}
