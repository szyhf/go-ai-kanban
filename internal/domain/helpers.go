package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UUID is a 16-byte UUID type.
// SQLite stores as BLOB; JSON serializes as RFC4122 string.
type UUID [16]byte

// NewUUID generates a new v4 UUID.
func NewUUID() UUID {
	return UUID(uuid.New())
}

// ParseUUID parses a UUID string in standard format.
func ParseUUID(s string) (UUID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return UUID{}, fmt.Errorf("invalid UUID: %w", err)
	}
	return UUID(u), nil
}

// MustParseUUID parses a UUID string or panics.
func MustParseUUID(s string) UUID {
	u, err := ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return u
}

// String returns the UUID in standard format "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx".
func (u UUID) String() string {
	return uuid.UUID(u).String()
}

// MarshalJSON implements json.Marshaler.
func (u UUID) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (u *UUID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("UUID must be a string: %w", err)
	}
	parsed, err := ParseUUID(s)
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}

// Scan implements sql.Scanner for SQLite BLOB storage.
func (u *UUID) Scan(value any) error {
	if value == nil {
		return fmt.Errorf("cannot scan NULL into non-nullable UUID")
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("UUID: expected []byte, got %T", value)
	}
	if len(b) != 16 {
		return fmt.Errorf("UUID: expected 16 bytes, got %d", len(b))
	}
	copy(u[:], b)
	return nil
}

// Value implements driver.Valuer for SQLite BLOB storage.
func (u UUID) Value() (driver.Value, error) {
	return u[:], nil
}

// UUIDPtr returns a pointer to the given UUID.
func UUIDPtr(u UUID) *UUID { return &u }

// TimePtr returns a pointer to the given time.
func TimePtr(t time.Time) *time.Time { return &t }

// StringPtr returns a pointer to the given string.
func StringPtr(s string) *string { return &s }

// Int64Ptr returns a pointer to the given int64.
func Int64Ptr(i int64) *int64 { return &i }
