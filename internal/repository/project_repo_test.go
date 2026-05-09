package repository

import (
	"testing"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/database"
	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

func testDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestProjectRepo_Create(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewProjectRepo(db.DB)

	project := &domain.Project{
		ID:   domain.NewUUID(),
		Name: "Test Project",
	}

	if err := repo.Create(project); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(project.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID returned nil, expected project")
	}

	if found.Name != project.Name {
		t.Errorf("Name: got %q, want %q", found.Name, project.Name)
	}
	if found.ID != project.ID {
		t.Errorf("ID: got %s, want %s", found.ID, project.ID)
	}
	if found.DefaultAgentWorkingDir != nil {
		t.Errorf("DefaultAgentWorkingDir: got %v, want nil", found.DefaultAgentWorkingDir)
	}
	if found.RemoteProjectID != nil {
		t.Errorf("RemoteProjectID: got %v, want nil", found.RemoteProjectID)
	}
	if found.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if found.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestProjectRepo_CreateWithOptionalFields(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewProjectRepo(db.DB)

	workingDir := "/home/user/project"
	remoteID := domain.NewUUID()

	project := &domain.Project{
		ID:                     domain.NewUUID(),
		Name:                   "Project With Fields",
		DefaultAgentWorkingDir: &workingDir,
		RemoteProjectID:        &remoteID,
	}

	if err := repo.Create(project); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(project.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID returned nil")
	}

	if found.DefaultAgentWorkingDir == nil || *found.DefaultAgentWorkingDir != workingDir {
		t.Errorf("DefaultAgentWorkingDir: got %v, want %q", found.DefaultAgentWorkingDir, workingDir)
	}
	if found.RemoteProjectID == nil || *found.RemoteProjectID != remoteID {
		t.Errorf("RemoteProjectID: got %v, want %s", found.RemoteProjectID, remoteID)
	}
}

func TestProjectRepo_FindAll(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewProjectRepo(db.DB)

	p1 := &domain.Project{ID: domain.NewUUID(), Name: "Zebra Project"}
	p2 := &domain.Project{ID: domain.NewUUID(), Name: "Alpha Project"}

	if err := repo.Create(p1); err != nil {
		t.Fatalf("Create p1: %v", err)
	}
	if err := repo.Create(p2); err != nil {
		t.Fatalf("Create p2: %v", err)
	}

	projects, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("FindAll: got %d projects, want 2", len(projects))
	}

	// Verify ordered by name: Alpha before Zebra.
	if projects[0].Name != "Alpha Project" {
		t.Errorf("projects[0].Name: got %q, want %q", projects[0].Name, "Alpha Project")
	}
	if projects[1].Name != "Zebra Project" {
		t.Errorf("projects[1].Name: got %q, want %q", projects[1].Name, "Zebra Project")
	}
}

func TestProjectRepo_FindByID_NotFound(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewProjectRepo(db.DB)

	found, err := repo.FindByID(domain.NewUUID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent project")
	}
}

func TestProjectRepo_Update(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewProjectRepo(db.DB)

	project := &domain.Project{
		ID:   domain.NewUUID(),
		Name: "Original Name",
	}
	if err := repo.Create(project); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Fetch to get timestamps.
	saved, err := repo.FindByID(project.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	// Brief pause so updated_at changes.
	time.Sleep(10 * time.Millisecond)

	saved.Name = "Updated Name"
	if err := repo.Update(saved); err != nil {
		t.Fatalf("Update: %v", err)
	}

	found, err := repo.FindByID(project.ID)
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}
	if found.Name != "Updated Name" {
		t.Errorf("Name: got %q, want %q", found.Name, "Updated Name")
	}
}

func TestProjectRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewProjectRepo(db.DB)

	project := &domain.Project{
		ID:   domain.NewUUID(),
		Name: "To Delete",
	}
	if err := repo.Create(project); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(project.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	found, err := repo.FindByID(project.ID)
	if err != nil {
		t.Fatalf("FindByID after delete: %v", err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}

func TestProjectRepo_Delete_NotFound(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewProjectRepo(db.DB)

	// Deleting a non-existent project should not error.
	if err := repo.Delete(domain.NewUUID()); err != nil {
		t.Fatalf("Delete non-existent: %v", err)
	}
}
