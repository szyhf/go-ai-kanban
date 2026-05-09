package repository

import (
	"testing"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

func TestTagRepo_Create(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewTagRepo(db.DB)

	tag := &domain.Tag{
		ID:        domain.NewUUID(),
		TagName:   "test-tag",
		Content:   "some content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := repo.Create(tag); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(tag.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID returned nil, expected tag")
	}

	if found.ID != tag.ID {
		t.Errorf("ID: got %s, want %s", found.ID, tag.ID)
	}
	if found.TagName != tag.TagName {
		t.Errorf("TagName: got %q, want %q", found.TagName, tag.TagName)
	}
	if found.Content != tag.Content {
		t.Errorf("Content: got %q, want %q", found.Content, tag.Content)
	}
	if found.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if found.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestTagRepo_FindAll(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewTagRepo(db.DB)

	names := []string{"beta", "alpha", "gamma"}
	for _, name := range names {
		tag := &domain.Tag{
			ID:        domain.NewUUID(),
			TagName:   name,
			Content:   "content for " + name,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Create(tag); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	tags, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	// Migrations seed 3 default tags, so total = 3 default + 3 test = 6.
	if len(tags) != 6 {
		t.Fatalf("FindAll: got %d tags, want 6", len(tags))
	}

	// Verify our 3 test tags appear in alphabetical order among the results.
	// The full sorted order includes default tags too.
	testNames := map[string]bool{"alpha": true, "beta": true, "gamma": true}
	var testTags []domain.Tag
	for _, tag := range tags {
		if testNames[tag.TagName] {
			testTags = append(testTags, tag)
		}
	}
	expected := []string{"alpha", "beta", "gamma"}
	for i, want := range expected {
		if testTags[i].TagName != want {
			t.Errorf("testTags[%d].TagName: got %q, want %q", i, testTags[i].TagName, want)
		}
	}
}

func TestTagRepo_Update(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewTagRepo(db.DB)

	tag := &domain.Tag{
		ID:        domain.NewUUID(),
		TagName:   "original-name",
		Content:   "original content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Create(tag); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Fetch persisted tag to get DB-generated timestamps.
	saved, err := repo.FindByID(tag.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	saved.TagName = "updated-name"
	saved.Content = "updated content"

	if err := repo.Update(saved); err != nil {
		t.Fatalf("Update: %v", err)
	}

	found, err := repo.FindByID(tag.ID)
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}
	if found.TagName != "updated-name" {
		t.Errorf("TagName: got %q, want %q", found.TagName, "updated-name")
	}
	if found.Content != "updated content" {
		t.Errorf("Content: got %q, want %q", found.Content, "updated content")
	}
}

func TestTagRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewTagRepo(db.DB)

	tag := &domain.Tag{
		ID:        domain.NewUUID(),
		TagName:   "to-delete",
		Content:   "delete me",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Create(tag); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(tag.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	found, err := repo.FindByID(tag.ID)
	if err != nil {
		t.Fatalf("FindByID after delete: %v", err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}

func TestTagRepo_FindByID_NotFound(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewTagRepo(db.DB)

	found, err := repo.FindByID(domain.NewUUID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent tag")
	}
}
