package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// Scratch represents a scratchpad entry.
// Matches Rust crates/db/src/models/scratch.rs.
// In the DB, scratch_type and payload are separate columns.
// In the API, they are nested under payload: {type, data}.
type Scratch struct {
	ID        UUID           `json:"id"`
	Payload   ScratchPayload `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// ScratchPayload represents the adjacently tagged payload.
// Matches Rust #[serde(tag = "type", content = "data", rename_all = "SCREAMING_SNAKE_CASE")].
// JSON: {"type": "DRAFT_TASK", "data": "..."} or {"type": "DRAFT_FOLLOW_UP", "data": {...}}
type ScratchPayload struct {
	Type ScratchType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

// CreateScratch is the request DTO for creating a scratch entry.
type CreateScratch struct {
	Payload ScratchPayload `json:"payload"`
}

// UpdateScratch is the request DTO for updating a scratch entry.
type UpdateScratch struct {
	Payload ScratchPayload `json:"payload"`
}

// --- Scratch payload data types ---

// DraftFollowUpData is the data for a DRAFT_FOLLOW_UP scratch.
type DraftFollowUpData struct {
	Message        string          `json:"message"`
	ExecutorConfig json.RawMessage `json:"executor_config"`
}

// PreviewSettingsData is the data for a PREVIEW_SETTINGS scratch.
type PreviewSettingsData struct {
	URL             string  `json:"url"`
	ScreenSize      *string `json:"screen_size"`
	ResponsiveWidth *int32  `json:"responsive_width"`
	ResponsiveHeight *int32 `json:"responsive_height"`
}

// WorkspaceNotesData is the data for a WORKSPACE_NOTES scratch.
type WorkspaceNotesData struct {
	Content string `json:"content"`
}

// WorkspacePanelStateData represents panel state within UI preferences.
type WorkspacePanelStateData struct {
	RightMainPanelMode    *string `json:"right_main_panel_mode,omitempty"`
	IsLeftMainPanelVisible bool   `json:"is_left_main_panel_visible"`
}

// WorkspacePrFilterData is the PR filter enum for workspace filters.
// Matches Rust rename_all = "snake_case".
type WorkspacePrFilterData string

const (
	WorkspacePrFilterAll   WorkspacePrFilterData = "all"
	WorkspacePrFilterHasPr WorkspacePrFilterData = "has_pr"
	WorkspacePrFilterNoPr  WorkspacePrFilterData = "no_pr"
)

// WorkspaceSortByData is the sort-by enum for workspace sorting.
// Matches Rust rename_all = "snake_case".
type WorkspaceSortByData string

const (
	WorkspaceSortByUpdatedAt WorkspaceSortByData = "updated_at"
	WorkspaceSortByCreatedAt WorkspaceSortByData = "created_at"
)

// WorkspaceSortOrderData is the sort order enum.
// Matches Rust rename_all = "snake_case".
type WorkspaceSortOrderData string

const (
	WorkspaceSortOrderAsc  WorkspaceSortOrderData = "asc"
	WorkspaceSortOrderDesc WorkspaceSortOrderData = "desc"
)

// WorkspaceFilterStateData holds workspace filter state.
type WorkspaceFilterStateData struct {
	ProjectIDs []string              `json:"project_ids"`
	PrFilter   WorkspacePrFilterData `json:"pr_filter"`
}

// WorkspaceSortStateData holds workspace sort state.
type WorkspaceSortStateData struct {
	SortBy    WorkspaceSortByData   `json:"sort_by"`
	SortOrder WorkspaceSortOrderData `json:"sort_order"`
}

// UIPreferencesData is the data for a UI_PREFERENCES scratch.
type UIPreferencesData struct {
	RepoActions                      map[string]string                `json:"repo_actions"`
	Expanded                         map[string]bool                  `json:"expanded"`
	ContextBarPosition               *string                          `json:"context_bar_position"`
	PaneSizes                        map[string]json.RawMessage       `json:"pane_sizes"`
	CollapsedPaths                   map[string][]string              `json:"collapsed_paths"`
	FileSearchRepoID                 *string                          `json:"file_search_repo_id"`
	IsLeftSidebarVisible             *bool                            `json:"is_left_sidebar_visible"`
	IsRightSidebarVisible            *bool                            `json:"is_right_sidebar_visible"`
	IsTerminalVisible                *bool                            `json:"is_terminal_visible"`
	WorkspacePanelStates             map[string]WorkspacePanelStateData `json:"workspace_panel_states"`
	WorkspaceFilters                 WorkspaceFilterStateData         `json:"workspace_filters"`
	WorkspaceSort                    WorkspaceSortStateData           `json:"workspace_sort"`
	SelectedOrgID                    *string                          `json:"selected_org_id"`
	SelectedProjectID                *string                          `json:"selected_project_id"`
	CreateDraftWorkspaceByDefault    *bool                            `json:"create_draft_workspace_by_default"`
	KanbanProjectViewSelections      map[string]json.RawMessage       `json:"kanban_project_view_selections"`
	KanbanProjectViewPreferences     map[string]json.RawMessage       `json:"kanban_project_view_preferences"`
}

// DraftWorkspaceRepo represents a repo selection in a draft workspace.
type DraftWorkspaceRepo struct {
	RepoID       UUID   `json:"repo_id"`
	TargetBranch string `json:"target_branch"`
}

// DraftWorkspaceLinkedIssue represents a linked issue in a draft workspace.
type DraftWorkspaceLinkedIssue struct {
	IssueID        string `json:"issue_id"`
	SimpleID       string `json:"simple_id"`
	Title          string `json:"title"`
	RemoteProjectID string `json:"remote_project_id"`
}

// DraftWorkspaceAttachment represents an attachment in a draft workspace.
type DraftWorkspaceAttachment struct {
	ID           UUID     `json:"id"`
	FilePath     string   `json:"file_path"`
	OriginalName string   `json:"original_name"`
	MimeType     *string  `json:"mime_type"`
	SizeBytes    int64    `json:"size_bytes"`
}

// DraftWorkspaceData is the data for a DRAFT_WORKSPACE scratch.
type DraftWorkspaceData struct {
	Message        string                    `json:"message"`
	Repos          []DraftWorkspaceRepo      `json:"repos"`
	ExecutorConfig json.RawMessage           `json:"executor_config"`
	LinkedIssue    *DraftWorkspaceLinkedIssue `json:"linked_issue"`
	Attachments    []DraftWorkspaceAttachment `json:"attachments"`
}

// ProjectRepoDefaultsData is the data for a PROJECT_REPO_DEFAULTS scratch.
type ProjectRepoDefaultsData struct {
	Repos []DraftWorkspaceRepo `json:"repos"`
}

// DraftIssueData is the data for a DRAFT_ISSUE scratch.
type DraftIssueData struct {
	Title                string   `json:"title"`
	Description          *string  `json:"description"`
	StatusID             string   `json:"status_id"`
	Priority             *string  `json:"priority"`
	AssigneeIDs          []string `json:"assignee_ids"`
	TagIDs               []string `json:"tag_ids"`
	CreateDraftWorkspace bool     `json:"create_draft_workspace"`
	ProjectID            string   `json:"project_id"`
	ParentIssueID        *string  `json:"parent_issue_id"`
}

// --- ScratchPayload typed accessors ---

// AsDraftTask returns the string data if type is DRAFT_TASK.
func (p *ScratchPayload) AsDraftTask() (string, error) {
	if p.Type != ScratchTypeDraftTask {
		return "", fmt.Errorf("scratch payload type is %s, not DRAFT_TASK", p.Type)
	}
	var s string
	if err := json.Unmarshal(p.Data, &s); err != nil {
		return "", fmt.Errorf("draft task data: %w", err)
	}
	return s, nil
}

// AsDraftFollowUp returns the DraftFollowUpData if type is DRAFT_FOLLOW_UP.
func (p *ScratchPayload) AsDraftFollowUp() (*DraftFollowUpData, error) {
	if p.Type != ScratchTypeDraftFollowUp {
		return nil, fmt.Errorf("scratch payload type is %s, not DRAFT_FOLLOW_UP", p.Type)
	}
	var d DraftFollowUpData
	if err := json.Unmarshal(p.Data, &d); err != nil {
		return nil, fmt.Errorf("draft follow up data: %w", err)
	}
	return &d, nil
}

// AsDraftWorkspace returns the DraftWorkspaceData if type is DRAFT_WORKSPACE.
func (p *ScratchPayload) AsDraftWorkspace() (*DraftWorkspaceData, error) {
	if p.Type != ScratchTypeDraftWorkspace {
		return nil, fmt.Errorf("scratch payload type is %s, not DRAFT_WORKSPACE", p.Type)
	}
	var d DraftWorkspaceData
	if err := json.Unmarshal(p.Data, &d); err != nil {
		return nil, fmt.Errorf("draft workspace data: %w", err)
	}
	return &d, nil
}

// AsDraftIssue returns the DraftIssueData if type is DRAFT_ISSUE.
func (p *ScratchPayload) AsDraftIssue() (*DraftIssueData, error) {
	if p.Type != ScratchTypeDraftIssue {
		return nil, fmt.Errorf("scratch payload type is %s, not DRAFT_ISSUE", p.Type)
	}
	var d DraftIssueData
	if err := json.Unmarshal(p.Data, &d); err != nil {
		return nil, fmt.Errorf("draft issue data: %w", err)
	}
	return &d, nil
}

// AsPreviewSettings returns the PreviewSettingsData if type is PREVIEW_SETTINGS.
func (p *ScratchPayload) AsPreviewSettings() (*PreviewSettingsData, error) {
	if p.Type != ScratchTypePreviewSettings {
		return nil, fmt.Errorf("scratch payload type is %s, not PREVIEW_SETTINGS", p.Type)
	}
	var d PreviewSettingsData
	if err := json.Unmarshal(p.Data, &d); err != nil {
		return nil, fmt.Errorf("preview settings data: %w", err)
	}
	return &d, nil
}

// AsWorkspaceNotes returns the WorkspaceNotesData if type is WORKSPACE_NOTES.
func (p *ScratchPayload) AsWorkspaceNotes() (*WorkspaceNotesData, error) {
	if p.Type != ScratchTypeWorkspaceNotes {
		return nil, fmt.Errorf("scratch payload type is %s, not WORKSPACE_NOTES", p.Type)
	}
	var d WorkspaceNotesData
	if err := json.Unmarshal(p.Data, &d); err != nil {
		return nil, fmt.Errorf("workspace notes data: %w", err)
	}
	return &d, nil
}

// AsUIPreferences returns the UIPreferencesData if type is UI_PREFERENCES.
func (p *ScratchPayload) AsUIPreferences() (*UIPreferencesData, error) {
	if p.Type != ScratchTypeUIPreferences {
		return nil, fmt.Errorf("scratch payload type is %s, not UI_PREFERENCES", p.Type)
	}
	var d UIPreferencesData
	if err := json.Unmarshal(p.Data, &d); err != nil {
		return nil, fmt.Errorf("ui preferences data: %w", err)
	}
	return &d, nil
}

// AsProjectRepoDefaults returns the ProjectRepoDefaultsData if type is PROJECT_REPO_DEFAULTS.
func (p *ScratchPayload) AsProjectRepoDefaults() (*ProjectRepoDefaultsData, error) {
	if p.Type != ScratchTypeProjectRepoDefaults {
		return nil, fmt.Errorf("scratch payload type is %s, not PROJECT_REPO_DEFAULTS", p.Type)
	}
	var d ProjectRepoDefaultsData
	if err := json.Unmarshal(p.Data, &d); err != nil {
		return nil, fmt.Errorf("project repo defaults data: %w", err)
	}
	return &d, nil
}
