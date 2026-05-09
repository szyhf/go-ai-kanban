package repository

import (
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// strPtrPtr returns a **string pointing to the given string value.
// Used for the double-option pattern in UpdateRepo.
func strPtrPtr(s string) **string {
	p := &s
	return &p
}

// nilStrPtr returns a **string pointing to nil (used to set a field to NULL).
func nilStrPtr() **string {
	var p *string
	return &p
}

func TestGitRepoRepo_FindOrCreate_New(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewGitRepoRepo(db.DB)

	got, err := repo.FindOrCreate("/path/to/repo", "my-repo", "My Repo")
	if err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}
	if got == nil {
		t.Fatal("FindOrCreate returned nil")
	}

	if got.Path != "/path/to/repo" {
		t.Errorf("Path: got %q, want %q", got.Path, "/path/to/repo")
	}
	if got.Name != "my-repo" {
		t.Errorf("Name: got %q, want %q", got.Name, "my-repo")
	}
	if got.DisplayName != "My Repo" {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, "My Repo")
	}
	if got.ID == (domain.UUID{}) {
		t.Error("ID should not be zero")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestGitRepoRepo_FindOrCreate_Existing(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewGitRepoRepo(db.DB)

	first, err := repo.FindOrCreate("/path/to/repo", "name1", "Display 1")
	if err != nil {
		t.Fatalf("FindOrCreate first: %v", err)
	}

	second, err := repo.FindOrCreate("/path/to/repo", "name2", "Display 2")
	if err != nil {
		t.Fatalf("FindOrCreate second: %v", err)
	}

	if second == nil {
		t.Fatal("second FindOrCreate returned nil")
	}

	if second.ID != first.ID {
		t.Errorf("ID mismatch: first=%s, second=%s", first.ID, second.ID)
	}
	if second.Name != first.Name {
		t.Errorf("Name should be unchanged: got %q, want %q", second.Name, first.Name)
	}
	if second.DisplayName != first.DisplayName {
		t.Errorf("DisplayName should be unchanged: got %q, want %q", second.DisplayName, first.DisplayName)
	}
}

func TestGitRepoRepo_FindByPath(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewGitRepoRepo(db.DB)

	got, err := repo.FindByPath("/nonexistent")
	if err != nil {
		t.Fatalf("FindByPath nonexistent: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent path")
	}

	created, err := repo.FindOrCreate("/some/path", "some-repo", "Some Repo")
	if err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}

	found, err := repo.FindByPath("/some/path")
	if err != nil {
		t.Fatalf("FindByPath: %v", err)
	}
	if found == nil {
		t.Fatal("FindByPath returned nil")
	}
	if found.ID != created.ID {
		t.Errorf("ID: got %s, want %s", found.ID, created.ID)
	}
}

func TestGitRepoRepo_FindAll(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewGitRepoRepo(db.DB)

	repos, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll empty: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}

	r1, err := repo.FindOrCreate("/path/z", "z-repo", "Z Repo")
	if err != nil {
		t.Fatalf("FindOrCreate z: %v", err)
	}
	r2, err := repo.FindOrCreate("/path/a", "a-repo", "A Repo")
	if err != nil {
		t.Fatalf("FindOrCreate a: %v", err)
	}

	repos, err = repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}

	if repos[0].ID != r2.ID {
		t.Errorf("repos[0]: got name %q, want %q", repos[0].Name, r2.Name)
	}
	if repos[1].ID != r1.ID {
		t.Errorf("repos[1]: got name %q, want %q", repos[1].Name, r1.Name)
	}
}

func TestGitRepoRepo_Update(t *testing.T) {
	t.Run("set value", func(t *testing.T) {
		db := testDB(t)
		t.Cleanup(func() { db.Close() })

		repo := NewGitRepoRepo(db.DB)

		created, err := repo.FindOrCreate("/path/to/repo", "repo", "Original")
		if err != nil {
			t.Fatalf("FindOrCreate: %v", err)
		}

		err = repo.Update(created.ID, domain.UpdateRepo{
			DisplayName: strPtrPtr("Updated Display"),
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}

		found, err := repo.FindByID(created.ID)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if found.DisplayName != "Updated Display" {
			t.Errorf("DisplayName: got %q, want %q", found.DisplayName, "Updated Display")
		}
	})

	t.Run("set to nil clears value", func(t *testing.T) {
		db := testDB(t)
		t.Cleanup(func() { db.Close() })

		repo := NewGitRepoRepo(db.DB)

		created, err := repo.FindOrCreate("/path/to/repo", "repo", "Original")
		if err != nil {
			t.Fatalf("FindOrCreate: %v", err)
		}

		err = repo.Update(created.ID, domain.UpdateRepo{
			SetupScript: strPtrPtr("some script"),
		})
		if err != nil {
			t.Fatalf("Update set: %v", err)
		}

		found, err := repo.FindByID(created.ID)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if found.SetupScript == nil || *found.SetupScript != "some script" {
			t.Errorf("SetupScript: got %v, want %q", found.SetupScript, "some script")
		}

		err = repo.Update(created.ID, domain.UpdateRepo{
			SetupScript: nilStrPtr(),
		})
		if err != nil {
			t.Fatalf("Update clear: %v", err)
		}

		found, err = repo.FindByID(created.ID)
		if err != nil {
			t.Fatalf("FindByID after clear: %v", err)
		}
		if found.SetupScript != nil {
			t.Errorf("SetupScript: got %v, want nil", found.SetupScript)
		}
	})

	t.Run("nil field does not update", func(t *testing.T) {
		db := testDB(t)
		t.Cleanup(func() { db.Close() })

		repo := NewGitRepoRepo(db.DB)

		created, err := repo.FindOrCreate("/path/to/repo", "repo", "Original")
		if err != nil {
			t.Fatalf("FindOrCreate: %v", err)
		}

		err = repo.Update(created.ID, domain.UpdateRepo{})
		if err != nil {
			t.Fatalf("Update no-op: %v", err)
		}

		found, err := repo.FindByID(created.ID)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if found.DisplayName != "Original" {
			t.Errorf("DisplayName: got %q, want %q", found.DisplayName, "Original")
		}
	})

	t.Run("not found returns error", func(t *testing.T) {
		db := testDB(t)
		t.Cleanup(func() { db.Close() })

		repo := NewGitRepoRepo(db.DB)

		err := repo.Update(domain.NewUUID(), domain.UpdateRepo{
			DisplayName: strPtrPtr("x"),
		})
		if err == nil {
			t.Fatal("expected error for non-existent repo")
		}
	})
}

func TestGitRepoRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewGitRepoRepo(db.DB)

	created, err := repo.FindOrCreate("/path/to/repo", "repo", "Repo")
	if err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}

	if err := repo.Delete(created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	found, err := repo.FindByID(created.ID)
	if err != nil {
		t.Fatalf("FindByID after delete: %v", err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}
