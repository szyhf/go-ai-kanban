package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// Row is an interface for database rows that can be scanned (database/sql.Row or .Rows).
type Row interface {
	Scan(dest ...any) error
}

// scanUUID scans a nullable UUID BLOB column.
func scanUUID(row Row, dest *domain.UUID) error {
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
func scanNullableUUID(row Row, dest **domain.UUID) error {
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
func scanNullableString(row Row, dest **string) error {
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
func scanNullableInt64(row Row, dest **int64) error {
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
func scanNullableTime(row Row, dest **time.Time) error {
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

// nullValue returns a driver.Value for a nullable pointer of any type.
// For simple pointer dereference, the generic form avoids boxing overhead.
func nullValue[T any](v *T) any {
	if v == nil {
		return nil
	}
	return *v
}

// nullUUID returns a driver.Value for a nullable UUID pointer.
// It converts the UUID to a byte slice for SQLite BLOB storage.
func nullUUID(u *domain.UUID) any {
	if u == nil {
		return nil
	}
	return u[:]
}

// nullTime returns a driver.Value for a nullable time pointer.
// It formats the time as RFC3339Nano for SQLite TEXT storage.
func nullTime(t *time.Time) any {
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
	"2006-01-02 15:04:05.999999999", // SQLite datetime('now','subsec')
	"2006-01-02 15:04:05-07:00",     // space + timezone offset no frac
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05",           // SQLite datetime('now')
	"2006-01-02T15:04:05.999999999", // ISO without Z
	"2006-01-02T15:04:05",           // ISO without Z no fraction
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

func (ts timeScanner) Scan(val any) error {
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

func (nts nullTimeScanner) Scan(val any) error {
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
