package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

func TestWorkspaceRepo_Create(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewWorkspaceRepo(db.DB)
	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	project := &domain.Project{ID: domain.NewUUID(), Name: "ws-test-project"}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: project.ID,
		Title:     "ws-test-task",
		Status:    domain.TaskStatusTodo,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	workspace := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/test-branch",
	}

	if err := repo.Create(workspace); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(workspace.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID returned nil, expected workspace")
	}

	if found.Branch != workspace.Branch {
		t.Errorf("Branch: got %q, want %q", found.Branch, workspace.Branch)
	}
	if found.TaskID == nil || *found.TaskID != task.ID {
		t.Errorf("TaskID: got %v, want %s", found.TaskID, task.ID)
	}
	if found.Archived {
		t.Error("Archived: got true, want false")
	}
	if found.Pinned {
		t.Error("Pinned: got true, want false")
	}
	if found.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if found.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestWorkspaceRepo_FindAll(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewWorkspaceRepo(db.DB)
	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	project := &domain.Project{ID: domain.NewUUID(), Name: "ws-findall-project"}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: project.ID,
		Title:     "ws-findall-task",
		Status:    domain.TaskStatusTodo,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	w1 := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/first",
	}
	if err := repo.Create(w1); err != nil {
		t.Fatalf("Create w1: %v", err)
	}

	// Small delay to ensure different updated_at ordering.
	time.Sleep(10 * time.Millisecond)

	w2 := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/second",
	}
	if err := repo.Create(w2); err != nil {
		t.Fatalf("Create w2: %v", err)
	}

	workspaces, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	if len(workspaces) != 2 {
		t.Fatalf("FindAll: got %d workspaces, want 2", len(workspaces))
	}

	// Verify ordered by updated_at DESC (most recent first).
	if workspaces[0].ID != w2.ID {
		t.Errorf("workspaces[0].ID: got %s, want %s (most recent)", workspaces[0].ID, w2.ID)
	}
	if workspaces[1].ID != w1.ID {
		t.Errorf("workspaces[1].ID: got %s, want %s (older)", workspaces[1].ID, w1.ID)
	}
}

func TestWorkspaceRepo_FindAllWithStatus(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsRepo := NewWorkspaceRepo(db.DB)
	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)
	sessionRepo := NewSessionRepo(db.DB)
	execRepo := NewExecutionProcessRepo(db.DB)

	project := &domain.Project{ID: domain.NewUUID(), Name: "ws-status-project"}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: project.ID,
		Title:     "ws-status-task",
		Status:    domain.TaskStatusTodo,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Workspace with a running execution process.
	wsRunning := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/running",
	}
	if err := wsRepo.Create(wsRunning); err != nil {
		t.Fatalf("Create wsRunning: %v", err)
	}

	session := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: wsRunning.ID,
	}
	if err := sessionRepo.Create(session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	ep := &domain.ExecutionProcess{
		ID:             domain.NewUUID(),
		SessionID: session.ID,
		RunReason: domain.RunReasonCodingAgent,
		ExecutorAction: json.RawMessage(`{}`),
		Status:    domain.ExecStatusRunning,
		StartedAt: time.Now(),
	}
	if err := execRepo.Create(ep); err != nil {
		t.Fatalf("create execution process: %v", err)
	}

	// Workspace with a failed execution process (codingagent).
	wsErrored := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/errored",
	}
	if err := wsRepo.Create(wsErrored); err != nil {
		t.Fatalf("Create wsErrored: %v", err)
	}

	session2 := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: wsErrored.ID,
	}
	if err := sessionRepo.Create(session2); err != nil {
		t.Fatalf("create session2: %v", err)
	}

	ep2 := &domain.ExecutionProcess{
		ID:        domain.NewUUID(),
		SessionID: session2.ID,
		RunReason: domain.RunReasonCodingAgent,
		Status:    domain.ExecStatusFailed,
		ExecutorAction: json.RawMessage(`{}`),
		StartedAt: time.Now(),
	}
	if err := execRepo.Create(ep2); err != nil {
		t.Fatalf("create execution process 2: %v", err)
	}

	// Workspace with no sessions.
	wsIdle := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/idle",
	}
	if err := wsRepo.Create(wsIdle); err != nil {
		t.Fatalf("Create wsIdle: %v", err)
	}

	results, err := wsRepo.FindAllWithStatus(nil, nil)
	if err != nil {
		t.Fatalf("FindAllWithStatus: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("FindAllWithStatus: got %d results, want 3", len(results))
	}

	// Build a map for easy lookup.
	byID := make(map[domain.UUID]domain.WorkspaceWithStatus)
	for _, r := range results {
		byID[r.ID] = r
	}

	// Verify running workspace.
	if ws, ok := byID[wsRunning.ID]; !ok {
		t.Error("wsRunning not found in results")
	} else if !ws.IsRunning {
		t.Error("wsRunning.IsRunning: got false, want true")
	}

	// Verify errored workspace.
	if ws, ok := byID[wsErrored.ID]; !ok {
		t.Error("wsErrored not found in results")
	} else if !ws.IsErrored {
		t.Error("wsErrored.IsErrored: got false, want true")
	}

	// Verify idle workspace.
	if ws, ok := byID[wsIdle.ID]; !ok {
		t.Error("wsIdle not found in results")
	} else {
		if ws.IsRunning {
			t.Error("wsIdle.IsRunning: got true, want false")
		}
		if ws.IsErrored {
			t.Error("wsIdle.IsErrored: got true, want false")
		}
	}
}

func TestWorkspaceRepo_FindByBranch(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewWorkspaceRepo(db.DB)
	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	project := &domain.Project{ID: domain.NewUUID(), Name: "ws-branch-project"}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: project.ID,
		Title:     "ws-branch-task",
		Status:    domain.TaskStatusTodo,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	branch := "feature/unique-branch-" + domain.NewUUID().String()[:8]
	workspace := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: branch,
	}
	if err := repo.Create(workspace); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByBranch(branch)
	if err != nil {
		t.Fatalf("FindByBranch: %v", err)
	}
	if found == nil {
		t.Fatal("FindByBranch returned nil, expected workspace")
	}
	if found.ID != workspace.ID {
		t.Errorf("ID: got %s, want %s", found.ID, workspace.ID)
	}
	if found.Branch != branch {
		t.Errorf("Branch: got %q, want %q", found.Branch, branch)
	}

	// Non-existent branch.
	notFound, err := repo.FindByBranch("nonexistent-branch")
	if err != nil {
		t.Fatalf("FindByBranch nonexistent: %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for non-existent branch")
	}
}

func TestWorkspaceRepo_Update(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewWorkspaceRepo(db.DB)
	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	project := &domain.Project{ID: domain.NewUUID(), Name: "ws-update-project"}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: project.ID,
		Title:     "ws-update-task",
		Status:    domain.TaskStatusTodo,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	workspace := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/original",
	}
	if err := repo.Create(workspace); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Fetch to get DB-generated timestamps.
	saved, err := repo.FindByID(workspace.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	newContainer := "container-abc123"
	saved.Branch = "feature/updated"
	saved.ContainerRef = &newContainer

	if err := repo.Update(saved); err != nil {
		t.Fatalf("Update: %v", err)
	}

	found, err := repo.FindByID(workspace.ID)
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}
	if found.Branch != "feature/updated" {
		t.Errorf("Branch: got %q, want %q", found.Branch, "feature/updated")
	}
	if found.ContainerRef == nil || *found.ContainerRef != newContainer {
		t.Errorf("ContainerRef: got %v, want %q", found.ContainerRef, newContainer)
	}
}

func TestWorkspaceRepo_UpdateArchived(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewWorkspaceRepo(db.DB)
	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	project := &domain.Project{ID: domain.NewUUID(), Name: "ws-archive-project"}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: project.ID,
		Title:     "ws-archive-task",
		Status:    domain.TaskStatusTodo,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	workspace := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/to-archive",
	}
	if err := repo.Create(workspace); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify initially not archived.
	found, err := repo.FindByID(workspace.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Archived {
		t.Error("Archived: got true, want false initially")
	}

	// Archive it.
	if err := repo.UpdateArchived(workspace.ID, true); err != nil {
		t.Fatalf("UpdateArchived: %v", err)
	}

	found, err = repo.FindByID(workspace.ID)
	if err != nil {
		t.Fatalf("FindByID after archive: %v", err)
	}
	if !found.Archived {
		t.Error("Archived: got false, want true after UpdateArchived")
	}

	// Unarchive it.
	if err := repo.UpdateArchived(workspace.ID, false); err != nil {
		t.Fatalf("UpdateArchived(false): %v", err)
	}

	found, err = repo.FindByID(workspace.ID)
	if err != nil {
		t.Fatalf("FindByID after unarchive: %v", err)
	}
	if found.Archived {
		t.Error("Archived: got true, want false after unarchive")
	}
}

func TestWorkspaceRepo_UpdateSetupCompleted(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewWorkspaceRepo(db.DB)
	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	project := &domain.Project{ID: domain.NewUUID(), Name: "ws-setup-project"}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: project.ID,
		Title:     "ws-setup-task",
		Status:    domain.TaskStatusTodo,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	workspace := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/setup-test",
	}
	if err := repo.Create(workspace); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify initially nil.
	found, err := repo.FindByID(workspace.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.SetupCompletedAt != nil {
		t.Errorf("SetupCompletedAt: got %v, want nil initially", found.SetupCompletedAt)
	}

	// Mark setup completed.
	if err := repo.UpdateSetupCompleted(workspace.ID); err != nil {
		t.Fatalf("UpdateSetupCompleted: %v", err)
	}

	found, err = repo.FindByID(workspace.ID)
	if err != nil {
		t.Fatalf("FindByID after setup completed: %v", err)
	}
	if found.SetupCompletedAt == nil {
		t.Fatal("SetupCompletedAt: got nil, want non-nil after UpdateSetupCompleted")
	}
	if found.SetupCompletedAt.IsZero() {
		t.Error("SetupCompletedAt should not be zero")
	}
}

func TestWorkspaceRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewWorkspaceRepo(db.DB)
	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	project := &domain.Project{ID: domain.NewUUID(), Name: "ws-delete-project"}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: project.ID,
		Title:     "ws-delete-task",
		Status:    domain.TaskStatusTodo,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	workspace := &domain.Workspace{
		ID:     domain.NewUUID(),
		TaskID: domain.UUIDPtr(task.ID),
		Branch: "feature/to-delete",
	}
	if err := repo.Create(workspace); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(workspace.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	found, err := repo.FindByID(workspace.ID)
	if err != nil {
		t.Fatalf("FindByID after delete: %v", err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}
