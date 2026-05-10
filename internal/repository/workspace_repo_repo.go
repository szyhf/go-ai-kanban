package repository

import (
	"database/sql"
	"fmt"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// WorkspaceRepoRepo provides data access for workspace-repo associations.
type WorkspaceRepoRepo struct {
	db *sql.DB
}

// NewWorkspaceRepoRepo creates a new WorkspaceRepoRepo.
func NewWorkspaceRepoRepo(db *sql.DB) *WorkspaceRepoRepo {
	return &WorkspaceRepoRepo{db: db}
}

// FindByWorkspaceID returns all workspace-repo associations for a workspace.
func (r *WorkspaceRepoRepo) FindByWorkspaceID(workspaceID domain.UUID) ([]domain.WorkspaceRepo, error) {
	rows, err := r.db.Query(`
		SELECT id, workspace_id, repo_id, target_branch, created_at, updated_at
		FROM workspace_repos
		WHERE workspace_id = ?
	`, workspaceID[:])
	if err != nil {
		return nil, fmt.Errorf("find workspace repos: %w", err)
	}
	defer rows.Close()

	var result []domain.WorkspaceRepo
	for rows.Next() {
		var wr domain.WorkspaceRepo
		if err := rows.Scan(
			&wr.ID, &wr.WorkspaceID, &wr.RepoID, &wr.TargetBranch,
			timeScanner{&wr.CreatedAt}, timeScanner{&wr.UpdatedAt},
		); err != nil {
			return nil, fmt.Errorf("scan workspace repo: %w", err)
		}
		result = append(result, wr)
	}
	return result, rows.Err()
}

// FindByWorkspaceIDWithRepos returns all repos associated with a workspace,
// joined with the target_branch from the workspace_repos junction table.
func (r *WorkspaceRepoRepo) FindByWorkspaceIDWithRepos(workspaceID domain.UUID) ([]domain.RepoWithTargetBranch, error) {
	rows, err := r.db.Query(`
		SELECT r.id, r.path, r.name, r.display_name,
		       r.setup_script, r.cleanup_script, r.archive_script, r.copy_files,
		       r.parallel_setup_script, r.dev_server_script,
		       r.default_target_branch, r.default_working_dir,
		       r.created_at, r.updated_at,
		       wr.target_branch
		FROM workspace_repos wr
		JOIN repos r ON r.id = wr.repo_id
		WHERE wr.workspace_id = ?
	`, workspaceID[:])
	if err != nil {
		return nil, fmt.Errorf("find workspace repos with details: %w", err)
	}
	defer rows.Close()

	var result []domain.RepoWithTargetBranch
	for rows.Next() {
		var rwt domain.RepoWithTargetBranch
		if err := scanRepoWithTargetBranch(rows, &rwt); err != nil {
			return nil, err
		}
		result = append(result, rwt)
	}
	return result, rows.Err()
}

// scanRepoWithTargetBranch scans a Repo row plus target_branch.
func scanRepoWithTargetBranch(row Row, rwt *domain.RepoWithTargetBranch) error {
	var setupScript, cleanupScript, archiveScript, copyFiles, devServerScript, defaultTargetBranch, defaultWorkingDir []byte
	var parallelSetupScript int
	err := row.Scan(
		&rwt.ID, &rwt.Path, &rwt.Name, &rwt.DisplayName,
		&setupScript, &cleanupScript, &archiveScript, &copyFiles,
		&parallelSetupScript, &devServerScript,
		&defaultTargetBranch, &defaultWorkingDir,
		timeScanner{&rwt.CreatedAt}, timeScanner{&rwt.UpdatedAt},
		&rwt.TargetBranch,
	)
	if err != nil {
		return fmt.Errorf("scan repo with target branch: %w", err)
	}
	if setupScript != nil {
		rwt.SetupScript = new(string)
		*rwt.SetupScript = string(setupScript)
	}
	if cleanupScript != nil {
		rwt.CleanupScript = new(string)
		*rwt.CleanupScript = string(cleanupScript)
	}
	if archiveScript != nil {
		rwt.ArchiveScript = new(string)
		*rwt.ArchiveScript = string(archiveScript)
	}
	if copyFiles != nil {
		rwt.CopyFiles = new(string)
		*rwt.CopyFiles = string(copyFiles)
	}
	rwt.ParallelSetupScript = parallelSetupScript != 0
	if devServerScript != nil {
		rwt.DevServerScript = new(string)
		*rwt.DevServerScript = string(devServerScript)
	}
	if defaultTargetBranch != nil {
		rwt.DefaultTargetBranch = new(string)
		*rwt.DefaultTargetBranch = string(defaultTargetBranch)
	}
	if defaultWorkingDir != nil {
		rwt.DefaultWorkingDir = new(string)
		*rwt.DefaultWorkingDir = string(defaultWorkingDir)
	}
	return nil
}

// CreateMany inserts multiple workspace-repo associations in a single transaction.
func (r *WorkspaceRepoRepo) CreateMany(workspaceID domain.UUID, repos []domain.CreateWorkspaceRepo) error {
	if len(repos) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction for create workspace repos: %w", err)
	}
	defer tx.Rollback()

	const stmtSQL = `
		INSERT INTO workspace_repos (id, workspace_id, repo_id, target_branch)
		VALUES (?, ?, ?, ?)
	`
	stmt, err := tx.Prepare(stmtSQL)
	if err != nil {
		return fmt.Errorf("prepare insert workspace repo: %w", err)
	}
	defer stmt.Close()

	for _, repo := range repos {
		id := domain.NewUUID()
		_, err := stmt.Exec(id[:], workspaceID[:], repo.RepoID[:], repo.TargetBranch)
		if err != nil {
			return fmt.Errorf("insert workspace repo: %w", err)
		}
	}

	return tx.Commit()
}

// DeleteByWorkspaceID deletes all workspace-repo associations for a workspace.
func (r *WorkspaceRepoRepo) DeleteByWorkspaceID(workspaceID domain.UUID) error {
	_, err := r.db.Exec(`DELETE FROM workspace_repos WHERE workspace_id = ?`, workspaceID[:])
	if err != nil {
		return fmt.Errorf("delete workspace repos: %w", err)
	}
	return nil
}

// DeleteByWorkspaceAndRepo deletes a specific workspace-repo association.
func (r *WorkspaceRepoRepo) DeleteByWorkspaceAndRepo(workspaceID, repoID domain.UUID) error {
	_, err := r.db.Exec(
		`DELETE FROM workspace_repos WHERE workspace_id = ? AND repo_id = ?`,
		workspaceID[:], repoID[:],
	)
	if err != nil {
		return fmt.Errorf("delete workspace repo association: %w", err)
	}
	return nil
}
