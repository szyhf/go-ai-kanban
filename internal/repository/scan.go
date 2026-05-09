package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// scanUUID scans a nullable UUID BLOB column.
func scanUUID(row interface{ Scan(...interface{}) error }, dest *domain.UUID) error {
	var b []byte
	if err := row.Scan(&b); err != nil {
		if err == sql.ErrNoRows {
			return err
		}
		return fmt.Errorf("scan UUID: %w", err)
	}
	if b == nil {
		return fmt.Errorf("scan UUID: got NULL")
	}
	if len(b) != 16 {
		return fmt.Errorf("scan UUID: expected 16 bytes, got %d", len(b))
	}
	copy(dest[:], b)
	return nil
}

// scanNullableUUID scans a nullable UUID BLOB column into *domain.UUID.
func scanNullableUUID(row interface{ Scan(...interface{}) error }, dest **domain.UUID) error {
	var b []byte
	if err := row.Scan(&b); err != nil {
		return fmt.Errorf("scan nullable UUID: %w", err)
	}
	if b == nil {
		*dest = nil
		return nil
	}
	if len(b) != 16 {
		return fmt.Errorf("scan nullable UUID: expected 16 bytes, got %d", len(b))
	}
	var u domain.UUID
	copy(u[:], b)
	*dest = &u
	return nil
}

// scanNullableString scans a nullable string column.
func scanNullableString(row interface{ Scan(...interface{}) error }, dest **string) error {
	var s sql.NullString
	if err := row.Scan(&s); err != nil {
		return fmt.Errorf("scan nullable string: %w", err)
	}
	if s.Valid {
		*dest = &s.String
	} else {
		*dest = nil
	}
	return nil
}

// scanNullableInt64 scans a nullable int64 column.
func scanNullableInt64(row interface{ Scan(...interface{}) error }, dest **int64) error {
	var n sql.NullInt64
	if err := row.Scan(&n); err != nil {
		return fmt.Errorf("scan nullable int64: %w", err)
	}
	if n.Valid {
		*dest = &n.Int64
	} else {
		*dest = nil
	}
	return nil
}

// scanNullableTime scans a nullable datetime column.
func scanNullableTime(row interface{ Scan(...interface{}) error }, dest **time.Time) error {
	var t sql.NullTime
	if err := row.Scan(&t); err != nil {
		return fmt.Errorf("scan nullable time: %w", err)
	}
	if t.Valid {
		*dest = &t.Time
	} else {
		*dest = nil
	}
	return nil
}

// nullUUID returns a driver.Value for a nullable UUID pointer.
func nullUUID(u *domain.UUID) interface{} {
	if u == nil {
		return nil
	}
	return u[:]
}

// nullString returns a driver.Value for a nullable string pointer.
func nullString(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

// nullInt64 returns a driver.Value for a nullable int64 pointer.
func nullInt64(n *int64) interface{} {
	if n == nil {
		return nil
	}
	return *n
}

// nullTime returns a driver.Value for a nullable time pointer.
func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// sqliteTimeFormats lists common time formats used by SQLite's datetime() function.
var sqliteTimeFormats = []string{
	"2006-01-02T15:04:05.999999999Z07:00", // RFC3339Nano with timezone
	"2006-01-02T15:04:05Z07:00",           // RFC3339 with timezone
	"2006-01-02T15:04:05.999999999Z",      // ISO with Z
	"2006-01-02T15:04:05Z",                // ISO with Z no fraction
	"2006-01-02 15:04:05.999999999-07:00", // space + Go timezone offset
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",       // SQLite datetime('now','subsec')
	"2006-01-02 15:04:05-07:00",           // space + timezone offset no frac
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05",                 // SQLite datetime('now')
	"2006-01-02T15:04:05.999999999",       // ISO without Z
	"2006-01-02T15:04:05",                 // ISO without Z no fraction
}

// parseDBTime parses a time string from SQLite into time.Time.
func parseDBTime(s string) (time.Time, error) {
	for _, f := range sqliteTimeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %q", s)
}

// timeScanner wraps a *time.Time and implements sql.Scanner for TEXT datetime columns.
// SQLite stores datetime as TEXT, which go-sqlite3 returns as string, not time.Time.
// Usage: rows.Scan(..., timeScanner{&p.CreatedAt}, timeScanner{&p.UpdatedAt})
type timeScanner struct{ ptr *time.Time }

func (ts timeScanner) Scan(val interface{}) error {
	if val == nil {
		return nil
	}
	s, ok := val.(string)
	if !ok {
		b, ok := val.([]byte)
		if !ok {
			return fmt.Errorf("timeScanner: expected string, got %T", val)
		}
		s = string(b)
	}
	t, err := parseDBTime(s)
	if err != nil {
		return err
	}
	*ts.ptr = t
	return nil
}

// nullTimeScanner wraps a **time.Time and implements sql.Scanner for nullable TEXT datetime columns.
// Usage: rows.Scan(..., nullTimeScanner{&p.SomeNullableTime})
type nullTimeScanner struct{ ptr **time.Time }

func (nts nullTimeScanner) Scan(val interface{}) error {
	if val == nil {
		*nts.ptr = nil
		return nil
	}
	s, ok := val.(string)
	if !ok {
		b, ok := val.([]byte)
		if !ok {
			return fmt.Errorf("nullTimeScanner: expected string, got %T", val)
		}
		s = string(b)
	}
	t, err := parseDBTime(s)
	if err != nil {
		return err
	}
	*nts.ptr = &t
	return nil
}
