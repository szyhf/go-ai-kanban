package repository

import (
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/database"
	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// createSessionTestFixtures sets up the project, task, and workspace required
// to create sessions, and returns the workspace ID.
func createSessionTestFixtures(t *testing.T, db *database.DB) domain.UUID {
	t.Helper()

	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)
	wsRepo := NewWorkspaceRepo(db.DB)

	project := &domain.Project{ID: domain.NewUUID(), Name: "session-test-project"}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: project.ID,
		Title:     "session-test-task",
		Status:    domain.TaskStatusTodo,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	workspace := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/session-test",
	}
	if err := wsRepo.Create(workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	return workspace.ID
}

func TestSessionRepo_Create(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewSessionRepo(db.DB)
	workspaceID := createSessionTestFixtures(t, db)

	session := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: workspaceID,
		Name:        domain.StringPtr("test-session"),
		Executor:    domain.StringPtr("claude"),
	}

	if err := repo.Create(session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(session.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID returned nil, expected session")
	}

	if found.WorkspaceID != workspaceID {
		t.Errorf("WorkspaceID: got %s, want %s", found.WorkspaceID, workspaceID)
	}
	if found.Name == nil || *found.Name != "test-session" {
		t.Errorf("Name: got %v, want %q", found.Name, "test-session")
	}
	if found.Executor == nil || *found.Executor != "claude" {
		t.Errorf("Executor: got %v, want %q", found.Executor, "claude")
	}
	if found.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if found.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestSessionRepo_FindByWorkspaceID(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewSessionRepo(db.DB)
	workspaceID := createSessionTestFixtures(t, db)

	s1 := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: workspaceID,
		Name:        domain.StringPtr("session-1"),
		Executor:    domain.StringPtr("claude"),
	}
	if err := repo.Create(s1); err != nil {
		t.Fatalf("Create s1: %v", err)
	}

	s2 := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: workspaceID,
		Name:        domain.StringPtr("session-2"),
		Executor:    domain.StringPtr("gpt"),
	}
	if err := repo.Create(s2); err != nil {
		t.Fatalf("Create s2: %v", err)
	}

	sessions, err := repo.FindByWorkspaceID(workspaceID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("FindByWorkspaceID: got %d sessions, want 2", len(sessions))
	}

	// Verify both sessions are present (order is by latest execution process,
	// falling back to created_at; with no execution processes, order is DESC by created_at).
	ids := make(map[domain.UUID]bool)
	for _, s := range sessions {
		ids[s.ID] = true
		if s.WorkspaceID != workspaceID {
			t.Errorf("WorkspaceID: got %s, want %s", s.WorkspaceID, workspaceID)
		}
	}
	if !ids[s1.ID] {
		t.Error("session 1 not found in results")
	}
	if !ids[s2.ID] {
		t.Error("session 2 not found in results")
	}
}

func TestSessionRepo_Update(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewSessionRepo(db.DB)
	workspaceID := createSessionTestFixtures(t, db)

	session := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: workspaceID,
		Name:        domain.StringPtr("original-name"),
		Executor:    domain.StringPtr("claude"),
	}
	if err := repo.Create(session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Fetch to get DB timestamps.
	saved, err := repo.FindByID(session.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	// Update fields.
	saved.Name = domain.StringPtr("updated-name")
	saved.Executor = domain.StringPtr("gpt")

	if err := repo.Update(saved); err != nil {
		t.Fatalf("Update: %v", err)
	}

	found, err := repo.FindByID(session.ID)
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}

	if found.Name == nil || *found.Name != "updated-name" {
		t.Errorf("Name: got %v, want %q", found.Name, "updated-name")
	}
	if found.Executor == nil || *found.Executor != "gpt" {
		t.Errorf("Executor: got %v, want %q", found.Executor, "gpt")
	}
}

func TestSessionRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewSessionRepo(db.DB)
	workspaceID := createSessionTestFixtures(t, db)

	session := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: workspaceID,
		Name:        domain.StringPtr("to-delete"),
	}
	if err := repo.Create(session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(session.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	found, err := repo.FindByID(session.ID)
	if err != nil {
		t.Fatalf("FindByID after delete: %v", err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}
