package repository

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// createTestSession inserts a minimal session row for the given workspace and returns its ID.
func createTestSession(t *testing.T, db *sql.DB, workspaceID domain.UUID) domain.UUID {
	t.Helper()
	id := domain.NewUUID()
	_, err := db.Exec(`
		INSERT INTO sessions (id, workspace_id, created_at, updated_at)
		VALUES (?, ?, datetime('now'), datetime('now'))
	`, id[:], workspaceID[:])
	if err != nil {
		t.Fatalf("insert test session: %v", err)
	}
	return id
}

func TestExecutionProcessRepo_Create_FindByID(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsID := createTestWorkspace(t, db.DB)
	sessionID := createTestSession(t, db.DB, wsID)
	repo := NewExecutionProcessRepo(db.DB)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ep := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID:      sessionID,
		RunReason:      domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{"type":"test"}`),
		Status:         domain.ExecStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := repo.Create(ep); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(ep.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID returned nil, expected execution process")
	}

	if found.ID != ep.ID {
		t.Errorf("ID: got %s, want %s", found.ID, ep.ID)
	}
	if found.SessionID != sessionID {
		t.Errorf("SessionID: got %s, want %s", found.SessionID, sessionID)
	}
	if found.RunReason != domain.RunReasonCodingAgent {
		t.Errorf("RunReason: got %q, want %q", found.RunReason, domain.RunReasonCodingAgent)
	}
	if found.Status != domain.ExecStatusRunning {
		t.Errorf("Status: got %q, want %q", found.Status, domain.ExecStatusRunning)
	}
	if found.ExitCode != nil {
		t.Errorf("ExitCode: got %v, want nil", found.ExitCode)
	}
	if found.Dropped {
		t.Errorf("Dropped: got true, want false")
	}
	if string(found.ExecutorAction) != `{"type":"test"}` {
		t.Errorf("ExecutorAction: got %s, want %s", found.ExecutorAction, `{"type":"test"}`)
	}
	if found.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
	if found.CompletedAt != nil {
		t.Errorf("CompletedAt: got %v, want nil", found.CompletedAt)
	}
}

func TestExecutionProcessRepo_FindBySessionID(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsID := createTestWorkspace(t, db.DB)
	sessionID := createTestSession(t, db.DB, wsID)
	repo := NewExecutionProcessRepo(db.DB)

	now := time.Now().UTC().Truncate(time.Microsecond)

	ep1 := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID:      sessionID,
		RunReason:      domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{"type":"test1"}`),
		Status:         domain.ExecStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	ep2 := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID:      sessionID,
		RunReason:      domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{"type":"test2"}`),
		Status:         domain.ExecStatusRunning,
		StartedAt:      now.Add(time.Second),
		CreatedAt:      now.Add(time.Second),
		UpdatedAt:      now.Add(time.Second),
	}

	if err := repo.Create(ep1); err != nil {
		t.Fatalf("Create ep1: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // ensure distinct DB-generated created_at for ORDER BY test
	if err := repo.Create(ep2); err != nil {
		t.Fatalf("Create ep2: %v", err)
	}

	// Both should be returned with includeDropped=true.
	procs, err := repo.FindBySessionID(sessionID, true)
	if err != nil {
		t.Fatalf("FindBySessionID includeDropped=true: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("FindBySessionID: got %d processes, want 2", len(procs))
	}

	// Order is created_at DESC, so ep2 should be first.
	if procs[0].ID != ep2.ID {
		t.Errorf("procs[0].ID: got %s, want %s (ep2, newer)", procs[0].ID, ep2.ID)
	}
	if procs[1].ID != ep1.ID {
		t.Errorf("procs[1].ID: got %s, want %s (ep1, older)", procs[1].ID, ep1.ID)
	}

	// Mark ep1 as dropped.
	if err := repo.UpdateDropped(ep1.ID, true); err != nil {
		t.Fatalf("UpdateDropped: %v", err)
	}

	// With includeDropped=false, only ep2 should be returned.
	procs, err = repo.FindBySessionID(sessionID, false)
	if err != nil {
		t.Fatalf("FindBySessionID includeDropped=false: %v", err)
	}
	if len(procs) != 1 {
		t.Fatalf("FindBySessionID includeDropped=false: got %d processes, want 1", len(procs))
	}
	if procs[0].ID != ep2.ID {
		t.Errorf("procs[0].ID: got %s, want %s", procs[0].ID, ep2.ID)
	}

	// With includeDropped=true, both should still be returned.
	procs, err = repo.FindBySessionID(sessionID, true)
	if err != nil {
		t.Fatalf("FindBySessionID includeDropped=true after drop: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("FindBySessionID includeDropped=true: got %d processes, want 2", len(procs))
	}
}

func TestExecutionProcessRepo_UpdateStatus(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsID := createTestWorkspace(t, db.DB)
	sessionID := createTestSession(t, db.DB, wsID)
	repo := NewExecutionProcessRepo(db.DB)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ep := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID:      sessionID,
		RunReason:      domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{"type":"test"}`),
		Status:         domain.ExecStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.Create(ep); err != nil {
		t.Fatalf("Create: %v", err)
	}

	exitCode := int64(0)
	if err := repo.UpdateStatus(ep.ID, domain.ExecStatusCompleted, &exitCode); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	found, err := repo.FindByID(ep.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Status != domain.ExecStatusCompleted {
		t.Errorf("Status: got %q, want %q", found.Status, domain.ExecStatusCompleted)
	}
	if found.ExitCode == nil || *found.ExitCode != 0 {
		t.Errorf("ExitCode: got %v, want 0", found.ExitCode)
	}
}

func TestExecutionProcessRepo_UpdateDropped(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsID := createTestWorkspace(t, db.DB)
	sessionID := createTestSession(t, db.DB, wsID)
	repo := NewExecutionProcessRepo(db.DB)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ep := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID:      sessionID,
		RunReason:      domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{"type":"test"}`),
		Status:         domain.ExecStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.Create(ep); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(ep.ID)
	if err != nil {
		t.Fatalf("FindByID before update: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID before update returned nil")
	}
	if found.Dropped {
		t.Error("Dropped should be false initially")
	}

	if err := repo.UpdateDropped(ep.ID, true); err != nil {
		t.Fatalf("UpdateDropped: %v", err)
	}

	found, err = repo.FindByID(ep.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !found.Dropped {
		t.Error("Dropped: got false, want true")
	}
}

func TestExecutionProcessRepoStateRepo_Create_Find(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsID := createTestWorkspace(t, db.DB)
	sessionID := createTestSession(t, db.DB, wsID)
	epRepo := NewExecutionProcessRepo(db.DB)
	stateRepo := NewExecutionProcessRepoStateRepo(db.DB)
	gitRepo := NewGitRepoRepo(db.DB)

	// Create execution process.
	now := time.Now().UTC().Truncate(time.Microsecond)
	ep := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID:      sessionID,
		RunReason:      domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{}`),
		Status:         domain.ExecStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := epRepo.Create(ep); err != nil {
		t.Fatalf("Create ep: %v", err)
	}

	// Create a git repo for the FK.
	repo, err := gitRepo.FindOrCreate("/path/test-repo", "test-repo", "Test Repo")
	if err != nil {
		t.Fatalf("FindOrCreate repo: %v", err)
	}

	beforeCommit := "abc123"
	state := &domain.ExecutionProcessRepoState{
		ID:                 domain.NewUUID(),
		ExecutionProcessID: ep.ID,
		RepoID:             repo.ID,
		BeforeHeadCommit:   &beforeCommit,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := stateRepo.Create(state); err != nil {
		t.Fatalf("Create state: %v", err)
	}

	states, err := stateRepo.FindByExecutionProcessID(ep.ID)
	if err != nil {
		t.Fatalf("FindByExecutionProcessID: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}

	s := states[0]
	if s.ID != state.ID {
		t.Errorf("ID: got %s, want %s", s.ID, state.ID)
	}
	if s.ExecutionProcessID != ep.ID {
		t.Errorf("ExecutionProcessID: got %s, want %s", s.ExecutionProcessID, ep.ID)
	}
	if s.RepoID != repo.ID {
		t.Errorf("RepoID: got %s, want %s", s.RepoID, repo.ID)
	}
	if s.BeforeHeadCommit == nil || *s.BeforeHeadCommit != "abc123" {
		t.Errorf("BeforeHeadCommit: got %v, want %q", s.BeforeHeadCommit, "abc123")
	}
	if s.AfterHeadCommit != nil {
		t.Errorf("AfterHeadCommit: got %v, want nil", s.AfterHeadCommit)
	}
}

func TestExecutionProcessRepoStateRepo_UpdateAfterHeadCommit(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsID := createTestWorkspace(t, db.DB)
	sessionID := createTestSession(t, db.DB, wsID)
	epRepo := NewExecutionProcessRepo(db.DB)
	stateRepo := NewExecutionProcessRepoStateRepo(db.DB)
	gitRepo := NewGitRepoRepo(db.DB)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ep := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID:      sessionID,
		RunReason:      domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{}`),
		Status:         domain.ExecStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := epRepo.Create(ep); err != nil {
		t.Fatalf("Create ep: %v", err)
	}

	repo, err := gitRepo.FindOrCreate("/path/test-repo2", "test-repo2", "Test Repo 2")
	if err != nil {
		t.Fatalf("FindOrCreate repo: %v", err)
	}

	state := &domain.ExecutionProcessRepoState{
		ID:                 domain.NewUUID(),
		ExecutionProcessID: ep.ID,
		RepoID:             repo.ID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := stateRepo.Create(state); err != nil {
		t.Fatalf("Create state: %v", err)
	}

	newCommit := "def456"
	if err := stateRepo.UpdateAfterHeadCommit(state.ID, newCommit); err != nil {
		t.Fatalf("UpdateAfterHeadCommit: %v", err)
	}

	states, err := stateRepo.FindByExecutionProcessID(ep.ID)
	if err != nil {
		t.Fatalf("FindByExecutionProcessID: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
	if states[0].AfterHeadCommit == nil || *states[0].AfterHeadCommit != "def456" {
		t.Errorf("AfterHeadCommit: got %v, want %q", states[0].AfterHeadCommit, "def456")
	}
}

func TestCodingAgentTurnRepo_Create_Find(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsID := createTestWorkspace(t, db.DB)
	sessionID := createTestSession(t, db.DB, wsID)
	epRepo := NewExecutionProcessRepo(db.DB)
	turnRepo := NewCodingAgentTurnRepo(db.DB)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ep := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID:      sessionID,
		RunReason:      domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{}`),
		Status:         domain.ExecStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := epRepo.Create(ep); err != nil {
		t.Fatalf("Create ep: %v", err)
	}

	agentSessionID := "sess-123"
	agentMessageID := "msg-456"
	prompt := "write a hello world"
	turn := &domain.CodingAgentTurn{
		ID:                 domain.NewUUID(),
		ExecutionProcessID: ep.ID,
		AgentSessionID:     &agentSessionID,
		AgentMessageID:     &agentMessageID,
		Prompt:             &prompt,
		Seen:               false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := turnRepo.Create(turn); err != nil {
		t.Fatalf("Create turn: %v", err)
	}

	turns, err := turnRepo.FindByExecutionProcessID(ep.ID)
	if err != nil {
		t.Fatalf("FindByExecutionProcessID: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}

	got := turns[0]
	if got.ID != turn.ID {
		t.Errorf("ID: got %s, want %s", got.ID, turn.ID)
	}
	if got.ExecutionProcessID != ep.ID {
		t.Errorf("ExecutionProcessID: got %s, want %s", got.ExecutionProcessID, ep.ID)
	}
	if got.AgentSessionID == nil || *got.AgentSessionID != agentSessionID {
		t.Errorf("AgentSessionID: got %v, want %q", got.AgentSessionID, agentSessionID)
	}
	if got.AgentMessageID == nil || *got.AgentMessageID != agentMessageID {
		t.Errorf("AgentMessageID: got %v, want %q", got.AgentMessageID, agentMessageID)
	}
	if got.Prompt == nil || *got.Prompt != prompt {
		t.Errorf("Prompt: got %v, want %q", got.Prompt, prompt)
	}
	if got.Summary != nil {
		t.Errorf("Summary: got %v, want nil", got.Summary)
	}
	if got.Seen {
		t.Error("Seen: got true, want false")
	}
}

func TestCodingAgentTurnRepo_UpdateSummaryAndSeen(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsID := createTestWorkspace(t, db.DB)
	sessionID := createTestSession(t, db.DB, wsID)
	epRepo := NewExecutionProcessRepo(db.DB)
	turnRepo := NewCodingAgentTurnRepo(db.DB)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ep := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID:      sessionID,
		RunReason:      domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{}`),
		Status:         domain.ExecStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := epRepo.Create(ep); err != nil {
		t.Fatalf("Create ep: %v", err)
	}

	turn := &domain.CodingAgentTurn{
		ID:                 domain.NewUUID(),
		ExecutionProcessID: ep.ID,
		Seen:               false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := turnRepo.Create(turn); err != nil {
		t.Fatalf("Create turn: %v", err)
	}

	// Verify initial state.
	turns, err := turnRepo.FindByExecutionProcessID(ep.ID)
	if err != nil {
		t.Fatalf("FindByExecutionProcessID initial: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn before update, got %d", len(turns))
	}
	if turns[0].Summary != nil {
		t.Errorf("Summary before update: got %v, want nil", turns[0].Summary)
	}
	if turns[0].Seen {
		t.Error("Seen before update: got true, want false")
	}

	// Update summary and seen.
	summary := "agent completed the task"
	if err := turnRepo.UpdateSummaryAndSeen(turn.ID, &summary, true); err != nil {
		t.Fatalf("UpdateSummaryAndSeen: %v", err)
	}

	turns, err = turnRepo.FindByExecutionProcessID(ep.ID)
	if err != nil {
		t.Fatalf("FindByExecutionProcessID after update: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].Summary == nil || *turns[0].Summary != summary {
		t.Errorf("Summary: got %v, want %q", turns[0].Summary, summary)
	}
	if !turns[0].Seen {
		t.Error("Seen: got false, want true")
	}
}
