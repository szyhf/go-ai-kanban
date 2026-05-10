package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Dialect represents the SQL dialect.
type Dialect string

const (
	DialectSQLite     Dialect = "sqlite"
	DialectPostgreSQL Dialect = "postgres"
)

// DB wraps a sql.DB with migration support.
type DB struct {
	*sql.DB
	dialect Dialect
	dsn     string
}

// Open creates a new database connection.
func Open(driver, dsn string) (*DB, error) {
	var dialect Dialect
	switch driver {
	case "sqlite", "":
		dialect = DialectSQLite
		driver = "sqlite3"
		if dsn == "" {
			return nil, fmt.Errorf("sqlite DSN is required")
		}
		// Ensure parent directory exists for SQLite.
		if idx := strings.LastIndex(dsn, "/"); idx > 0 {
			dir := dsn[:idx]
			if err := mkdirAll(dir); err != nil {
				return nil, fmt.Errorf("create db directory: %w", err)
			}
		}
	case "postgres":
		dialect = DialectPostgreSQL
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", driver, err)
	}

	// Connection pool settings.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// SQLite-specific settings.
	if dialect == DialectSQLite {
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return nil, fmt.Errorf("set WAL mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
			return nil, fmt.Errorf("enable foreign keys: %w", err)
		}
	}

	return &DB{DB: db, dialect: dialect, dsn: dsn}, nil
}

// Dialect returns the SQL dialect.
func (db *DB) Dialect() Dialect {
	return db.dialect
}

// Migrate runs all pending database migrations.
func (db *DB) Migrate() error {
	// Create migrations tracking table.
	if err := db.createMigrationTable(); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	// Read embedded migration files.
	files, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	// Sort files by name (they are already prefixed with timestamps).
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}

		applied, err := db.isMigrationApplied(f.Name())
		if err != nil {
			return fmt.Errorf("check migration %s: %w", f.Name(), err)
		}
		if applied {
			continue
		}

		content, err := migrationFS.ReadFile("migrations/" + f.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f.Name(), err)
		}

		slog.Info("正在应用迁移", "file", f.Name())
		if err := db.applyMigration(f.Name(), string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f.Name(), err)
		}
		slog.Info("迁移已应用", "file", f.Name())
	}

	return nil
}

func (db *DB) createMigrationTable() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)
	`)
	return err
}

func (db *DB) isMigrationApplied(name string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM _migrations WHERE name = ?", name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *DB) applyMigration(name, content string) error {
	// Use raw SQL transaction management instead of Go's sql.Tx.
	// Several migrations use multi-transaction patterns:
	//   COMMIT; PRAGMA foreign_keys=OFF; BEGIN TRANSACTION; ...
	// Go's sql.Tx cannot handle these patterns, so we manage transactions
	// at the SQL level, tracking state with inTx.
	inTx := false

	// Start an initial implicit transaction (matching sqlx behavior).
	// Migrations that begin with "COMMIT;" will end this transaction.
	if _, err := db.Exec("BEGIN TRANSACTION"); err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	inTx = true

	statements := splitSQL(content)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		upper := strings.ToUpper(stmt)

		var err error
		switch {
		case upper == "COMMIT" || strings.HasPrefix(upper, "COMMIT "):
			if inTx {
				_, err = db.Exec("COMMIT")
				if err == nil {
					inTx = false
				}
			}

		case upper == "BEGIN" || strings.HasPrefix(upper, "BEGIN "):
			if !inTx {
				_, err = db.Exec("BEGIN TRANSACTION")
				if err == nil {
					inTx = true
				}
			}

		case strings.HasPrefix(upper, "PRAGMA"):
			// PRAGMAs run as-is in the current transaction state.
			// foreign_keys pragmas: always after COMMIT, so inTx=false.
			// foreign_key_check: always inside BEGIN..COMMIT, so inTx=true.
			_, err = db.Exec(stmt)

		default:
			if !inTx {
				if _, e := db.Exec("BEGIN TRANSACTION"); e != nil {
					return fmt.Errorf("begin: %w", e)
				}
				inTx = true
			}
			_, err = db.Exec(stmt)
		}

		if err != nil {
			if inTx {
				db.Exec("ROLLBACK")
			}
			return fmt.Errorf("exec: %w\nstatement: %s", err, truncate(stmt, 200))
		}
	}

	// Commit any remaining open transaction.
	if inTx {
		if _, err := db.Exec("COMMIT"); err != nil {
			return fmt.Errorf("final commit: %w", err)
		}
	}

	// Record migration as applied.
	if _, err := db.Exec("INSERT INTO _migrations (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.DB.Close()
}

// splitSQL splits SQL content into individual statements,
// handling multi-line statements, comments, and BEGIN...END compound blocks
// (used in CREATE TRIGGER/FUNCTION).
func splitSQL(content string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	compoundDepth := 0 // Track BEGIN...END nesting in triggers/functions

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}

		upper := strings.ToUpper(trimmed)
		words := strings.Fields(upper)

		// Detect standalone BEGIN while building a statement (trigger/function compound block).
		// "BEGIN TRANSACTION" is NOT a compound start — it's transaction control.
		if compoundDepth == 0 && current.Len() > 0 && len(words) == 1 && words[0] == "BEGIN" {
			compoundDepth++
			for _, ch := range line {
				current.WriteRune(ch)
			}
			current.WriteRune('\n')
			continue
		}

		// Inside a compound block: accumulate without splitting on semicolons.
		if compoundDepth > 0 {
			for _, ch := range line {
				current.WriteRune(ch)
			}
			current.WriteRune('\n')

			// Check for END (closing the compound block).
			if len(words) > 0 {
				first := strings.TrimSuffix(words[0], ";")
				if first == "END" {
					compoundDepth--
					if compoundDepth == 0 && strings.Contains(trimmed, ";") {
						// Extract the complete trigger/function statement.
						stmt := strings.TrimSpace(current.String())
						stmt = strings.TrimSuffix(stmt, ";")
						stmt = strings.TrimSpace(stmt)
						if stmt != "" {
							statements = append(statements, stmt)
						}
						current.Reset()
					}
				}
			}
			continue
		}

		// Normal line: split on semicolons.
		for _, ch := range line {
			if ch == '\'' {
				inString = !inString
			}
			if ch == ';' && !inString {
				stmt := strings.TrimSpace(current.String())
				if stmt != "" {
					statements = append(statements, stmt)
				}
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		}
		current.WriteRune('\n')
	}

	if remaining := strings.TrimSpace(current.String()); remaining != "" {
		statements = append(statements, remaining)
	}

	return statements
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}
