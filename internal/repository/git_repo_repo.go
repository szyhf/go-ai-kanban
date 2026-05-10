package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// GitRepoRepo provides data access for git repos.
type GitRepoRepo struct {
	db *sql.DB
}

// NewGitRepoRepo creates a new GitRepoRepo.
func NewGitRepoRepo(db *sql.DB) *GitRepoRepo {
	return &GitRepoRepo{db: db}
}

// scanRepo scans a full Repo row from the current row position.
func scanRepo(row Row, r *domain.Repo) error {
	var setupScript, cleanupScript, archiveScript, copyFiles, devServerScript, defaultTargetBranch, defaultWorkingDir []byte
	var parallelSetupScript int
	err := row.Scan(
		&r.ID, &r.Path, &r.Name, &r.DisplayName,
		&setupScript, &cleanupScript, &archiveScript, &copyFiles,
		&parallelSetupScript, &devServerScript,
		&defaultTargetBranch, &defaultWorkingDir,
		timeScanner{&r.CreatedAt}, timeScanner{&r.UpdatedAt},
	)
	if err != nil {
		return fmt.Errorf("scan repo: %w", err)
	}
	if setupScript != nil {
		r.SetupScript = new(string)
		*r.SetupScript = string(setupScript)
	}
	if cleanupScript != nil {
		r.CleanupScript = new(string)
		*r.CleanupScript = string(cleanupScript)
	}
	if archiveScript != nil {
		r.ArchiveScript = new(string)
		*r.ArchiveScript = string(archiveScript)
	}
	if copyFiles != nil {
		r.CopyFiles = new(string)
		*r.CopyFiles = string(copyFiles)
	}
	r.ParallelSetupScript = parallelSetupScript != 0
	if devServerScript != nil {
		r.DevServerScript = new(string)
		*r.DevServerScript = string(devServerScript)
	}
	if defaultTargetBranch != nil {
		r.DefaultTargetBranch = new(string)
		*r.DefaultTargetBranch = string(defaultTargetBranch)
	}
	if defaultWorkingDir != nil {
		r.DefaultWorkingDir = new(string)
		*r.DefaultWorkingDir = string(defaultWorkingDir)
	}
	return nil
}

const repoColumns = `id, path, name, display_name, setup_script, cleanup_script, archive_script, copy_files, parallel_setup_script, dev_server_script, default_target_branch, default_working_dir, created_at, updated_at`

// FindAll returns all repos ordered by name.
func (r *GitRepoRepo) FindAll() ([]domain.Repo, error) {
	rows, err := r.db.Query(`SELECT ` + repoColumns + ` FROM repos ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("find all repos: %w", err)
	}
	defer rows.Close()

	var repos []domain.Repo
	for rows.Next() {
		var repo domain.Repo
		if err := scanRepo(rows, &repo); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

// FindByID returns a repo by its ID. Returns nil, nil if not found.
func (r *GitRepoRepo) FindByID(id domain.UUID) (*domain.Repo, error) {
	var repo domain.Repo
	err := r.db.QueryRow(`SELECT `+repoColumns+` FROM repos WHERE id = ?`, id[:]).Scan(
		&repo.ID, &repo.Path, &repo.Name, &repo.DisplayName,
		&repo.SetupScript, &repo.CleanupScript, &repo.ArchiveScript, &repo.CopyFiles,
		&repo.ParallelSetupScript, &repo.DevServerScript,
		&repo.DefaultTargetBranch, &repo.DefaultWorkingDir,
		timeScanner{&repo.CreatedAt}, timeScanner{&repo.UpdatedAt},
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find repo by id: %w", err)
	}
	return &repo, nil
}

// FindByPath returns a repo by its path. Returns nil, nil if not found.
func (r *GitRepoRepo) FindByPath(path string) (*domain.Repo, error) {
	var repo domain.Repo
	err := r.db.QueryRow(`SELECT `+repoColumns+` FROM repos WHERE path = ?`, path).Scan(
		&repo.ID, &repo.Path, &repo.Name, &repo.DisplayName,
		&repo.SetupScript, &repo.CleanupScript, &repo.ArchiveScript, &repo.CopyFiles,
		&repo.ParallelSetupScript, &repo.DevServerScript,
		&repo.DefaultTargetBranch, &repo.DefaultWorkingDir,
		timeScanner{&repo.CreatedAt}, timeScanner{&repo.UpdatedAt},
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find repo by path: %w", err)
	}
	return &repo, nil
}

// FindOrCreate returns an existing repo by path, or creates a new one.
// Uses INSERT OR IGNORE to handle the UNIQUE(path) constraint, then fetches the row.
func (r *GitRepoRepo) FindOrCreate(path, name, displayName string) (*domain.Repo, error) {
	id := domain.NewUUID()
	_, err := r.db.Exec(`
		INSERT OR IGNORE INTO repos (id, path, name, display_name)
		VALUES (?, ?, ?, ?)
	`, id[:], path, name, displayName)
	if err != nil {
		return nil, fmt.Errorf("find or create repo: %w", err)
	}
	return r.FindByPath(path)
}

// Update updates a repo using the double-option pattern:
// nil field = don't update, pointer to nil = set NULL, pointer to value = set value.
func (r *GitRepoRepo) Update(id domain.UUID, update domain.UpdateRepo) error {
	var setClauses []string
	var args []any

	if update.DisplayName != nil {
		setClauses = append(setClauses, "display_name = ?")
		args = append(args, nullValue(*update.DisplayName))
	}
	if update.SetupScript != nil {
		setClauses = append(setClauses, "setup_script = ?")
		args = append(args, nullValue(*update.SetupScript))
	}
	if update.CleanupScript != nil {
		setClauses = append(setClauses, "cleanup_script = ?")
		args = append(args, nullValue(*update.CleanupScript))
	}
	if update.ArchiveScript != nil {
		setClauses = append(setClauses, "archive_script = ?")
		args = append(args, nullValue(*update.ArchiveScript))
	}
	if update.CopyFiles != nil {
		setClauses = append(setClauses, "copy_files = ?")
		args = append(args, nullValue(*update.CopyFiles))
	}
	if update.ParallelSetupScript != nil {
		setClauses = append(setClauses, "parallel_setup_script = ?")
		if *update.ParallelSetupScript == nil {
			// false / default when nil bool
			args = append(args, false)
		} else {
			args = append(args, **update.ParallelSetupScript)
		}
	}
	if update.DevServerScript != nil {
		setClauses = append(setClauses, "dev_server_script = ?")
		args = append(args, nullValue(*update.DevServerScript))
	}
	if update.DefaultTargetBranch != nil {
		setClauses = append(setClauses, "default_target_branch = ?")
		args = append(args, nullValue(*update.DefaultTargetBranch))
	}
	if update.DefaultWorkingDir != nil {
		setClauses = append(setClauses, "default_working_dir = ?")
		args = append(args, nullValue(*update.DefaultWorkingDir))
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = datetime('now', 'subsec')")
	args = append(args, id[:])

	query := fmt.Sprintf("UPDATE repos SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update repo: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("update repo: no rows affected for id %s", id)
	}
	return nil
}

// Delete deletes a repo by ID.
func (r *GitRepoRepo) Delete(id domain.UUID) error {
	_, err := r.db.Exec(`DELETE FROM repos WHERE id = ?`, id[:])
	if err != nil {
		return fmt.Errorf("delete repo: %w", err)
	}
	return nil
}
