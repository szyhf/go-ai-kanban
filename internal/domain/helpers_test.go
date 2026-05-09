package domain

import (
	"encoding/json"
	"testing"
)

func TestUUIDJSONRoundTrip(t *testing.T) {
	original := NewUUID()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal UUID: %v", err)
	}

	// Should be a JSON string in RFC4122 format
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("UUID should serialize as string: %v", err)
	}
	if s != original.String() {
		t.Errorf("got %q, want %q", s, original.String())
	}

	// Round-trip
	var parsed UUID
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal UUID: %v", err)
	}
	if parsed != original {
		t.Errorf("round-trip failed: got %v, want %v", parsed, original)
	}
}

func TestUUIDInStruct(t *testing.T) {
	type testStruct struct {
		ID   UUID    `json:"id"`
		Name string  `json:"name"`
		Opt  *UUID   `json:"opt"`
	}

	id := NewUUID()
	ts := testStruct{ID: id, Name: "test"}

	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify null for nil pointer
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if raw["opt"] != nil {
		t.Errorf("expected nil opt, got %v", raw["opt"])
	}

	// Round-trip
	var ts2 testStruct
	if err := json.Unmarshal(data, &ts2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ts2.ID != id {
		t.Errorf("ID mismatch: got %v, want %v", ts2.ID, id)
	}
	if ts2.Opt != nil {
		t.Errorf("expected nil Opt, got %v", ts2.Opt)
	}
}

func TestParseUUID(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"6ba7b810-9dad-11d1-80b4-00c04fd430c8", true},
		{"6ba7b8109dad11d180b400c04fd430c8", true},
		{"{6ba7b810-9dad-11d1-80b4-00c04fd430c8}", true},
		{"invalid", false},
		{"", false},
		{"12345", false},
	}

	for _, tt := range tests {
		_, err := ParseUUID(tt.input)
		if (err == nil) != tt.valid {
			t.Errorf("ParseUUID(%q): valid=%v, err=%v", tt.input, tt.valid, err)
		}
	}
}

func TestUUIDScanValue(t *testing.T) {
	original := NewUUID()

	// Value
	val, err := original.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	b, ok := val.([]byte)
	if !ok {
		t.Fatalf("Value: expected []byte, got %T", val)
	}
	if len(b) != 16 {
		t.Fatalf("Value: expected 16 bytes, got %d", len(b))
	}

	// Scan
	var scanned UUID
	if err := scanned.Scan(b); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if scanned != original {
		t.Errorf("Scan round-trip failed: got %v, want %v", scanned, original)
	}

	// Scan nil should error (non-nullable)
	var nilUUID UUID
	if err := nilUUID.Scan(nil); err == nil {
		t.Error("Scan(nil) should error for non-nullable UUID")
	}
}
