// Package git provides Git operations for the AI Kanban application.
//
// This package uses a hybrid architecture:
//   - go-git/v5 (pure Go) for read-only graph queries (branch listing,
//     merge-base computation, ahead/behind counting, in-memory merge).
//   - git CLI (os/exec) for working-tree mutations (worktree add/remove,
//     commit, push, rebase, reset) because the CLI refuses to clobber
//     uncommitted changes, correctly handles sparse-checkout, and avoids
//     WSL/Windows repository corruption issues documented by the Rust team.
package git
