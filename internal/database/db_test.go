package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestOpenSQLite(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "test.db")

	db, err := Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Verify SQLite settings.
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("check journal_mode: %v", err)
	}
	if strings.ToLower(mode) != "wal" {
		t.Errorf("expected WAL mode, got %s", mode)
	}

	var fkEnabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("check foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Errorf("expected foreign_keys=1, got %d", fkEnabled)
	}

	// Verify file was created.
	if _, err := os.Stat(dsn); err != nil {
		t.Errorf("db file not created: %v", err)
	}
}

func TestOpenEmptyDSN(t *testing.T) {
	_, err := Open("sqlite", "")
	if err == nil {
		t.Error("expected error for empty DSN")
	}
}

func TestOpenUnsupportedDriver(t *testing.T) {
	_, err := Open("mysql", "localhost:3306")
	if err == nil {
		t.Error("expected error for unsupported driver")
	}
}

func TestMigrate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Verify _migrations table was created.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count == 0 {
		t.Error("expected migrations to be recorded")
	}

	// Verify key tables exist after all migrations.
	expectedTables := []string{
		"projects",
		"tasks",
		"workspaces",
		"sessions",
		"repos",
		"project_repos",
		"workspace_repos",
		"execution_processes",
		"execution_process_repo_states",
		"coding_agent_turns",
		"execution_process_logs",
		"merges",
		"pull_requests",
		"tags",
		"scratch",
		"attachments",
		"workspace_attachments",
		"task_attachments",
		"migration_state",
	}

	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}

	// Verify dropped tables do NOT exist.
	droppedTables := []string{
		"task_attempts",
		"task_attempt_activities",
		"task_templates",
		"executor_sessions",
		"drafts",
		"shared_tasks",
		"images",
		"task_images",
		"workspace_images",
		"attempt_repos",
	}

	for _, table := range droppedTables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err == nil {
			t.Errorf("dropped table %q should not exist", table)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Run migrations twice.
	if err := db.Migrate(); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	var firstCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&firstCount); err != nil {
		t.Fatalf("count first migrations: %v", err)
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	var secondCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&secondCount); err != nil {
		t.Fatalf("count second migrations: %v", err)
	}

	if firstCount != secondCount {
		t.Errorf("migrations not idempotent: first=%d, second=%d", firstCount, secondCount)
	}
}

func TestMigrateWithDomainInsert(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Test inserting a project using UUID BLOB.
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i)
	}

	_, err := db.Exec(
		"INSERT INTO projects (id, name) VALUES (?, ?)",
		id, "Test Project",
	)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}

	// Read it back.
	var name string
	if err := db.QueryRow("SELECT name FROM projects WHERE id = ?", id).Scan(&name); err != nil {
		t.Fatalf("select project: %v", err)
	}
	if name != "Test Project" {
		t.Errorf("got name %q, want %q", name, "Test Project")
	}

	// Test inserting a task.
	taskID := make([]byte, 16)
	for i := range taskID {
		taskID[i] = byte(i + 16)
	}

	_, err = db.Exec(
		"INSERT INTO tasks (id, project_id, title, status) VALUES (?, ?, ?, ?)",
		taskID, id, "Fix bug", "todo",
	)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}

	var status string
	if err := db.QueryRow("SELECT status FROM tasks WHERE id = ?", taskID).Scan(&status); err != nil {
		t.Fatalf("select task: %v", err)
	}
	if status != "todo" {
		t.Errorf("got status %q, want %q", status, "todo")
	}
}

func TestSplitSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single statement",
			input:    "SELECT 1;",
			expected: []string{"SELECT 1"},
		},
		{
			name:     "multiple statements",
			input:    "SELECT 1;\nSELECT 2;",
			expected: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:     "with comments",
			input:    "-- comment\nSELECT 1;\n-- another\nSELECT 2;",
			expected: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:     "semicolon in string",
			input:    "INSERT INTO t VALUES ('hello;world');",
			expected: []string{"INSERT INTO t VALUES ('hello;world')"},
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "only comments",
			input:    "-- comment 1\n-- comment 2\n",
			expected: nil,
		},
		{
			name:     "PRAGMA statements",
			input:    "PRAGMA foreign_keys = OFF;\nPRAGMA journal_mode=WAL;",
			expected: []string{"PRAGMA foreign_keys = OFF", "PRAGMA journal_mode=WAL"},
		},
		{
			name:  "multi-transaction pattern",
			input: "ALTER TABLE t ADD COLUMN c TEXT;\nCOMMIT;\nPRAGMA foreign_keys = OFF;\nBEGIN TRANSACTION;\nSELECT 1;",
			expected: []string{
				"ALTER TABLE t ADD COLUMN c TEXT",
				"COMMIT",
				"PRAGMA foreign_keys = OFF",
				"BEGIN TRANSACTION",
				"SELECT 1",
			},
		},
		{
			name:  "CREATE TRIGGER with BEGIN...END",
			input: "CREATE TRIGGER trg\nAFTER UPDATE ON t\nFOR EACH ROW\nBEGIN\n    UPDATE t SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;\nEND;",
			expected: []string{
				"CREATE TRIGGER trg\nAFTER UPDATE ON t\nFOR EACH ROW\nBEGIN\n    UPDATE t SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;\nEND",
			},
		},
		{
			name:  "trigger mixed with normal statements",
			input: "CREATE TABLE t (id INTEGER, updated_at TEXT);\nCREATE TRIGGER trg\nAFTER UPDATE ON t\nFOR EACH ROW\nBEGIN\n    UPDATE t SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;\nEND;\nINSERT INTO t VALUES (1, '2025-01-01');",
			expected: []string{
				"CREATE TABLE t (id INTEGER, updated_at TEXT)",
				"CREATE TRIGGER trg\nAFTER UPDATE ON t\nFOR EACH ROW\nBEGIN\n    UPDATE t SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;\nEND",
				"INSERT INTO t VALUES (1, '2025-01-01')",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitSQL(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("got %d statements, want %d\n got: %v\nwant: %v",
					len(result), len(tt.expected), result, tt.expected)
				return
			}
			for i, stmt := range result {
				if strings.TrimSpace(stmt) != strings.TrimSpace(tt.expected[i]) {
					t.Errorf("statement %d:\n got: %q\nwant: %q", i, stmt, tt.expected[i])
				}
			}
		})
	}
}

func TestDialect(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if db.Dialect() != DialectSQLite {
		t.Errorf("expected DialectSQLite, got %s", db.Dialect())
	}
}

// openTestDB creates an in-memory SQLite database for testing.
func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db
}

// TestMigrationOrder verifies migrations are applied in timestamp order.
func TestMigrationOrder(t *testing.T) {
	files, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var names []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
			names = append(names, f.Name())
		}
	}

	if !sort.IsSorted(sort.StringSlice(names)) {
		t.Errorf("migration files not sorted by name")
	}

	t.Logf("Total migrations: %d", len(names))
}

// TestApplyMigration tests the raw transaction management logic.
func TestApplyMigration(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create the migration tracking table.
	if err := db.createMigrationTable(); err != nil {
		t.Fatalf("createMigrationTable: %v", err)
	}

	// Test simple migration.
	if err := db.applyMigration("001_simple.sql",
		"CREATE TABLE test1 (id INTEGER PRIMARY KEY); INSERT INTO test1 VALUES (1);",
	); err != nil {
		t.Fatalf("apply simple migration: %v", err)
	}

	var val int
	if err := db.QueryRow("SELECT id FROM test1").Scan(&val); err != nil {
		t.Fatalf("query test1: %v", err)
	}
	if val != 1 {
		t.Errorf("got %d, want 1", val)
	}

	// Test multi-transaction migration.
	if err := db.applyMigration("002_complex.sql", `
CREATE TABLE test2 (id INTEGER PRIMARY KEY);
COMMIT;
PRAGMA foreign_keys = OFF;
BEGIN TRANSACTION;
CREATE TABLE test3 (id INTEGER PRIMARY KEY, ref INTEGER REFERENCES test2(id));
PRAGMA foreign_key_check;
COMMIT;
PRAGMA foreign_keys = ON;
BEGIN TRANSACTION;
`); err != nil {
		t.Fatalf("apply complex migration: %v", err)
	}

	// Verify all tables exist.
	for _, table := range []string{"test1", "test2", "test3"} {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}

	// Verify both migrations are recorded.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 migrations, got %d", count)
	}

	// Verify foreign keys are re-enabled.
	var fkEnabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("check foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Errorf("foreign_keys should be re-enabled, got %d", fkEnabled)
	}
}

// TestApplyMigrationRollback tests that failed migrations are rolled back.
func TestApplyMigrationRollback(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.createMigrationTable(); err != nil {
		t.Fatalf("createMigrationTable: %v", err)
	}

	// Migration that will fail.
	err := db.applyMigration("001_fail.sql", `
CREATE TABLE should_exist (id INTEGER PRIMARY KEY);
INSERT INTO nonexistent_table VALUES (1);
`)
	if err == nil {
		t.Fatal("expected error for invalid SQL")
	}

	// The first statement should have been rolled back.
	var name string
	err = db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='should_exist'",
	).Scan(&name)
	if err != sql.ErrNoRows {
		t.Errorf("table should not exist after rollback: err=%v, name=%s", err, name)
	}

	// Migration should not be recorded.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("failed migration should not be recorded, got %d", count)
	}
}
