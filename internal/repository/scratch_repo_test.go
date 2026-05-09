package repository

import (
	"encoding/json"
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

func TestScratchRepo_Upsert_New(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewScratchRepo(db.DB)

	scratch := &domain.Scratch{
		ID: domain.NewUUID(),
		Payload: domain.ScratchPayload{
			Type: domain.ScratchTypeDraftTask,
			Data: json.RawMessage(`"some draft text"`),
		},
	}

	if err := repo.Upsert(scratch); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	found, err := repo.FindByIDAndType(scratch.ID, scratch.Payload.Type)
	if err != nil {
		t.Fatalf("FindByIDAndType: %v", err)
	}
	if found == nil {
		t.Fatal("FindByIDAndType returned nil, expected scratch")
	}

	if found.ID != scratch.ID {
		t.Errorf("ID: got %s, want %s", found.ID, scratch.ID)
	}
	if found.Payload.Type != scratch.Payload.Type {
		t.Errorf("Payload.Type: got %q, want %q", found.Payload.Type, scratch.Payload.Type)
	}
	if string(found.Payload.Data) != string(scratch.Payload.Data) {
		t.Errorf("Payload.Data: got %s, want %s", found.Payload.Data, scratch.Payload.Data)
	}
	if found.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if found.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestScratchRepo_Upsert_Update(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewScratchRepo(db.DB)

	scratch := &domain.Scratch{
		ID: domain.NewUUID(),
		Payload: domain.ScratchPayload{
			Type: domain.ScratchTypeDraftTask,
			Data: json.RawMessage(`"original draft"`),
		},
	}

	if err := repo.Upsert(scratch); err != nil {
		t.Fatalf("Upsert initial: %v", err)
	}

	// Upsert again with same ID + type but different payload.
	scratch.Payload.Data = json.RawMessage(`"updated draft"`)
	if err := repo.Upsert(scratch); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}

	found, err := repo.FindByIDAndType(scratch.ID, scratch.Payload.Type)
	if err != nil {
		t.Fatalf("FindByIDAndType: %v", err)
	}
	if found == nil {
		t.Fatal("FindByIDAndType returned nil after upsert update")
	}

	if string(found.Payload.Data) != `"updated draft"` {
		t.Errorf("Payload.Data: got %s, want %q", found.Payload.Data, `"updated draft"`)
	}
}

func TestScratchRepo_FindAllByType(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewScratchRepo(db.DB)

	// Create scratches of different types.
	scratchTypes := []struct {
		scratchType domain.ScratchType
		data        string
	}{
		{domain.ScratchTypeDraftTask, `"task data"`},
		{domain.ScratchTypeDraftFollowUp, `{"message":"follow up"}`},
		{domain.ScratchTypeUIPreferences, `{"expanded":{}}`},
	}

	for _, st := range scratchTypes {
		s := &domain.Scratch{
			ID: domain.NewUUID(),
			Payload: domain.ScratchPayload{
				Type: st.scratchType,
				Data: json.RawMessage(st.data),
			},
		}
		if err := repo.Upsert(s); err != nil {
			t.Fatalf("Upsert %s: %v", st.scratchType, err)
		}
	}

	// FindAllByType for DRAFT_TASK should return only 1.
	results, err := repo.FindAllByType(domain.ScratchTypeDraftTask)
	if err != nil {
		t.Fatalf("FindAllByType: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("FindAllByType: got %d results, want 1", len(results))
	}
	if results[0].Payload.Type != domain.ScratchTypeDraftTask {
		t.Errorf("Payload.Type: got %q, want %q", results[0].Payload.Type, domain.ScratchTypeDraftTask)
	}

	// FindAllByType for non-existent type should return 0.
	results, err = repo.FindAllByType(domain.ScratchTypePreviewSettings)
	if err != nil {
		t.Fatalf("FindAllByType non-existent: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("FindAllByType non-existent: got %d results, want 0", len(results))
	}
}

func TestScratchRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewScratchRepo(db.DB)

	scratch := &domain.Scratch{
		ID: domain.NewUUID(),
		Payload: domain.ScratchPayload{
			Type: domain.ScratchTypeDraftFollowUp,
			Data: json.RawMessage(`{"message":"delete me"}`),
		},
	}
	if err := repo.Upsert(scratch); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if err := repo.Delete(scratch.ID, scratch.Payload.Type); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	found, err := repo.FindByIDAndType(scratch.ID, scratch.Payload.Type)
	if err != nil {
		t.Fatalf("FindByIDAndType after delete: %v", err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}

func TestScratchRepo_FindByIDAndType_NotFound(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewScratchRepo(db.DB)

	found, err := repo.FindByIDAndType(domain.NewUUID(), domain.ScratchTypeDraftTask)
	if err != nil {
		t.Fatalf("FindByIDAndType: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent scratch")
	}
}
