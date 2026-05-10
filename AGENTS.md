# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` — Go application entrypoints (`cmd/server`).
- `internal/domain/` — Domain models.
- `internal/repository/` — Data access layer (SQLite).
- `internal/database/` — Database connection and migrations.
- `internal/git/` — Git operations service.
- `internal/service/` — Business logic service layer.
- `internal/handler/` — API HTTP handlers (Chi router).
- `internal/executor/` — Claude Code executor.
- `packages/local-web/`: Local React + TypeScript app entrypoint (Vite, Tailwind). Shell source in `packages/local-web/src`.
- `packages/web-core/`: Shared React + TypeScript frontend library (`packages/web-core/src`).
- `shared/`: TypeScript types (`shared/types.ts`) and agent tool schemas (`shared/schemas/`). Manually maintained alongside the Go backend.
- `assets/`, `dev_assets_seed/`, `dev_assets/`: Packaged and local dev assets.
- `npx-cli/`: Files published to the npm CLI package.
- `scripts/`: Dev helpers.
- `docs/`: Documentation files.

### Package-specific guides
- [`docs/AGENTS.md`](docs/AGENTS.md) — Mintlify documentation writing guidelines and component reference.
- [`packages/local-web/AGENTS.md`](packages/local-web/AGENTS.md) — Web app design system styling guidelines.

## Managing Shared Types Between Go and TypeScript

`shared/types.ts` is manually maintained to match the Go backend struct definitions.
When changing API types, update both the Go struct (with correct `json` tags) in `internal/domain/` and the corresponding TypeScript type in `shared/types.ts`.

## Build, Test, and Development Commands
- Install: `pnpm i`
- Run dev (web app + backend with ports auto-assigned): `pnpm run dev`
- Backend (watch): `pnpm run backend:dev:watch`
- Web app (dev): `pnpm run local-web:dev`
- Type checks: `pnpm run check` (frontend) and `pnpm run backend:check` (Go build + vet)
- Go tests: `go test -race ./...`
- Go lint: `go vet ./...`
- Go format: `gofmt -w . && goimports -w .`
- Local NPX build: `pnpm run build:npx` then `pnpm pack` in `npx-cli/`
- Format code: `pnpm run format` (runs `gofmt` + `goimports` for Go + web-core/web Prettier)
- Lint: `pnpm run lint` (runs web/ui ESLint + `go vet`)

## Before Completing a Task
- Run `pnpm run format` to format all Go and web code.
- Run `go test -race ./...` to ensure tests pass.

## Coding Style & Naming Conventions
- Go: `gofmt` enforced; group imports by stdlib/external/internal; camelCase funcs, PascalCase exported types.
- TypeScript/React: ESLint + Prettier (2 spaces, single quotes, 80 cols). PascalCase components, camelCase vars/functions, kebab-case file names where practical.
- Keep functions small, add `json` tags on Go structs for API serialization.

## Testing Guidelines
- Go: prefer table-driven tests alongside code (`_test.go` files), run `go test -race ./...`. Add tests for new logic and edge cases.
- Web app: ensure `pnpm run check` and `pnpm run lint` pass. If adding runtime logic, include lightweight tests (e.g., Vitest) in the same directory.

## Security & Config Tips
- Use `.env` for local overrides; never commit secrets. Key envs: `FRONTEND_PORT`, `BACKEND_PORT`, `HOST`
- Dev ports and assets are managed by `scripts/setup-dev-environment.js`.

## Task Execution Workflow

Each task must follow this process:
1. **Pass tests** — Run `go test -race ./...` after each task.
2. **Full project audit** — Review changes for correctness.
3. **Git commit** — Commit after audit passes.
4. **Escalate decisions** — Ask user if unsure.
5. **Auto-continue** — Move to next task without waiting for confirmation.
