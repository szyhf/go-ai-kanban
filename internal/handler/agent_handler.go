package handler

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"
)

// registerAgentRoutes registers agent-related routes.
func (h *Handler) registerAgentRoutes(r chi.Router) {
	r.Get("/preset-options", h.handleAgentPresetOptions)
	r.Get("/discovered-options/ws", h.handleAgentDiscoveredOptionsWS)
	r.Get("/check-availability", h.handleAgentCheckAvailability)
}

// agentCLIConfig maps executor names to their CLI commands and display info.
var agentCLIConfigs = map[string]struct {
	Command     string
	DisplayName string
	InstallHint string
}{
	"CLAUDE_CODE":  {Command: "claude", DisplayName: "Claude Code", InstallHint: "npm install -g @anthropic-ai/claude-code"},
	"CODEX":        {Command: "codex", DisplayName: "Codex", InstallHint: "npm install -g @openai/codex"},
	"AIDER":        {Command: "aider", DisplayName: "Aider", InstallHint: "pip install aider-chat"},
	"CURSOR_AGENT": {Command: "cursor-agent", DisplayName: "Cursor Agent", InstallHint: "Install Cursor IDE"},
}

// handleAgentPresetOptions handles GET /api/agents/preset-options.
func (h *Handler) handleAgentPresetOptions(w http.ResponseWriter, r *http.Request) {
	// Return default Claude Code executor config.
	success(w, map[string]any{
		"executor": "CLAUDE_CODE",
		"variant":  nil,
	})
}

// handleAgentDiscoveredOptionsWS handles GET /api/agents/discovered-options/ws.
func (h *Handler) handleAgentDiscoveredOptionsWS(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{})
}

// handleAgentCheckAvailability handles GET /api/agents/check-availability.
func (h *Handler) handleAgentCheckAvailability(w http.ResponseWriter, r *http.Request) {
	executorName := getQuery(r, "executor")

	result := map[string]any{
		"available":    false,
		"version":      nil,
		"install_hint": nil,
	}

	if executorName == "" {
		executorName = "CLAUDE_CODE"
	}

	if cfg, ok := agentCLIConfigs[strings.ToUpper(executorName)]; ok {
		if path, err := exec.LookPath(cfg.Command); err == nil {
			result["available"] = true
			result["path"] = path
			// Try to get version
			out, err := exec.Command(cfg.Command, "--version").Output()
			if err == nil {
				version := strings.TrimSpace(string(out))
				// Take first line only
				if idx := strings.Index(version, "\n"); idx >= 0 {
					version = version[:idx]
				}
				result["version"] = version
			}
		} else {
			result["install_hint"] = cfg.InstallHint
		}
	}

	success(w, result)
}
