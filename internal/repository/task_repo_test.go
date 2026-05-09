package repository

import (
	"testing"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// createTestProject is a helper that creates a project and returns its UUID.
func createTestProject(t *testing.T, repo *ProjectRepo) domain.UUID {
	t.Helper()
	project := &domain.Project{
		ID:   domain.NewUUID(),
		Name: "Test Project for Tasks",
	}
	if err := repo.Create(project); err != nil {
		t.Fatalf("create test project: %v", err)
	}
	return project.ID
}

func TestTaskRepo_Create(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	projectID := createTestProject(t, projectRepo)
	now := time.Now().UTC().Truncate(time.Microsecond)

	task := &domain.Task{
		ID:          domain.NewUUID(),
		ProjectID:   projectID,
		Title:       "Implement feature X",
		Description: domain.StringPtr("Detailed description"),
		Status:      domain.TaskStatusTodo,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := taskRepo.FindByID(task.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID returned nil, expected task")
	}

	if found.Title != task.Title {
		t.Errorf("Title: got %q, want %q", found.Title, task.Title)
	}
	if found.ProjectID != projectID {
		t.Errorf("ProjectID: got %s, want %s", found.ProjectID, projectID)
	}
	if found.Description == nil || *found.Description != "Detailed description" {
		t.Errorf("Description: got %v, want %q", found.Description, "Detailed description")
	}
	if found.Status != domain.TaskStatusTodo {
		t.Errorf("Status: got %q, want %q", found.Status, domain.TaskStatusTodo)
	}
}

func TestTaskRepo_Create_WithOptionalFields(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	projectID := createTestProject(t, projectRepo)
	workspaceRepo := NewWorkspaceRepo(db.DB)

	// Create a workspace for the FK constraint.
	ws := &domain.Workspace{
		ID:     domain.NewUUID(),
		Branch: "test-branch",
	}
	if err := workspaceRepo.Create(ws); err != nil {
		t.Fatalf("create test workspace: %v", err)
	}
	parentWorkspaceID := ws.ID
	now := time.Now().UTC().Truncate(time.Microsecond)

	task := &domain.Task{
		ID:                domain.NewUUID(),
		ProjectID:         projectID,
		Title:             "Task with workspace",
		Status:            domain.TaskStatusInProgress,
		ParentWorkspaceID: &parentWorkspaceID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := taskRepo.FindByID(task.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID returned nil")
	}

	if found.ParentWorkspaceID == nil || *found.ParentWorkspaceID != parentWorkspaceID {
		t.Errorf("ParentWorkspaceID: got %v, want %s", found.ParentWorkspaceID, parentWorkspaceID)
	}
	if found.Status != domain.TaskStatusInProgress {
		t.Errorf("Status: got %q, want %q", found.Status, domain.TaskStatusInProgress)
	}
}

func TestTaskRepo_FindByProjectID(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	projectID := createTestProject(t, projectRepo)
	now := time.Now().UTC().Truncate(time.Microsecond)

	titles := []string{"Task A", "Task B", "Task C"}
	for _, title := range titles {
		task := &domain.Task{
			ID:        domain.NewUUID(),
			ProjectID: projectID,
			Title:     title,
			Status:    domain.TaskStatusTodo,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := taskRepo.Create(task); err != nil {
			t.Fatalf("Create %q: %v", title, err)
		}
		now = now.Add(time.Second)
	}

	tasks, err := taskRepo.FindByProjectID(projectID)
	if err != nil {
		t.Fatalf("FindByProjectID: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("FindByProjectID: got %d tasks, want 3", len(tasks))
	}

	// Verify ordered by created_at.
	gotTitles := make([]string, len(tasks))
	for i, t := range tasks {
		gotTitles[i] = t.Title
	}
	for i, want := range titles {
		if gotTitles[i] != want {
			t.Errorf("tasks[%d].Title: got %q, want %q", i, gotTitles[i], want)
		}
	}
}

func TestTaskRepo_FindByProjectID_Empty(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	projectID := createTestProject(t, projectRepo)

	tasks, err := taskRepo.FindByProjectID(projectID)
	if err != nil {
		t.Fatalf("FindByProjectID: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks for project with no tasks, got %d", len(tasks))
	}
}

func TestTaskRepo_FindByID_NotFound(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	taskRepo := NewTaskRepo(db.DB)

	found, err := taskRepo.FindByID(domain.NewUUID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent task")
	}
}

func TestTaskRepo_Update(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	projectID := createTestProject(t, projectRepo)
	now := time.Now().UTC().Truncate(time.Microsecond)

	task := &domain.Task{
		ID:          domain.NewUUID(),
		ProjectID:   projectID,
		Title:       "Original Title",
		Description: domain.StringPtr("Original description"),
		Status:      domain.TaskStatusTodo,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Fetch persisted task.
	saved, err := taskRepo.FindByID(task.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	// Update fields.
	saved.Title = "Updated Title"
	saved.Description = domain.StringPtr("Updated description")
	saved.Status = domain.TaskStatusDone

	if err := taskRepo.Update(saved); err != nil {
		t.Fatalf("Update: %v", err)
	}

	found, err := taskRepo.FindByID(task.ID)
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}
	if found.Title != "Updated Title" {
		t.Errorf("Title: got %q, want %q", found.Title, "Updated Title")
	}
	if found.Description == nil || *found.Description != "Updated description" {
		t.Errorf("Description: got %v, want %q", found.Description, "Updated description")
	}
	if found.Status != domain.TaskStatusDone {
		t.Errorf("Status: got %q, want %q", found.Status, domain.TaskStatusDone)
	}
}

func TestTaskRepo_UpdateStatus(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	projectID := createTestProject(t, projectRepo)
	now := time.Now().UTC().Truncate(time.Microsecond)

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: projectID,
		Title:     "Status change task",
		Status:    domain.TaskStatusTodo,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := taskRepo.UpdateStatus(task.ID, domain.TaskStatusInProgress); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	found, err := taskRepo.FindByID(task.ID)
	if err != nil {
		t.Fatalf("FindByID after UpdateStatus: %v", err)
	}
	if found.Status != domain.TaskStatusInProgress {
		t.Errorf("Status: got %q, want %q", found.Status, domain.TaskStatusInProgress)
	}
}

func TestTaskRepo_UpdateStatus_TableDriven(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)
	projectID := createTestProject(t, projectRepo)

	tests := []struct {
		name   string
		status domain.TaskStatus
	}{
		{"todo", domain.TaskStatusTodo},
		{"inprogress", domain.TaskStatusInProgress},
		{"inreview", domain.TaskStatusInReview},
		{"done", domain.TaskStatusDone},
		{"cancelled", domain.TaskStatusCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Microsecond)
			task := &domain.Task{
				ID:        domain.NewUUID(),
				ProjectID: projectID,
				Title:     "Task " + tt.name,
				Status:    domain.TaskStatusTodo,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := taskRepo.Create(task); err != nil {
				t.Fatalf("Create: %v", err)
			}

			if err := taskRepo.UpdateStatus(task.ID, tt.status); err != nil {
				t.Fatalf("UpdateStatus: %v", err)
			}

			found, err := taskRepo.FindByID(task.ID)
			if err != nil {
				t.Fatalf("FindByID: %v", err)
			}
			if found.Status != tt.status {
				t.Errorf("Status: got %q, want %q", found.Status, tt.status)
			}
		})
	}
}

func TestTaskRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	projectRepo := NewProjectRepo(db.DB)
	taskRepo := NewTaskRepo(db.DB)

	projectID := createTestProject(t, projectRepo)
	now := time.Now().UTC().Truncate(time.Microsecond)

	task := &domain.Task{
		ID:        domain.NewUUID(),
		ProjectID: projectID,
		Title:     "To be deleted",
		Status:    domain.TaskStatusTodo,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := taskRepo.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := taskRepo.Delete(task.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	found, err := taskRepo.FindByID(task.ID)
	if err != nil {
		t.Fatalf("FindByID after delete: %v", err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}

func TestTaskRepo_Delete_NotFound(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	taskRepo := NewTaskRepo(db.DB)

	// Deleting a non-existent task should not error.
	if err := taskRepo.Delete(domain.NewUUID()); err != nil {
		t.Fatalf("Delete non-existent: %v", err)
	}
}
