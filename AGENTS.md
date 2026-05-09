# Repository Guidelines

## Project Structure & Module Organization
- `crates/`: Rust workspace crates — `server` (API + bins), `db` (SQLx models/migrations), `executors`, `services`, `utils`, `git` (Git operations), `api-types` (shared API types for local + remote), `review` (PR review tool), `deployment`, `local-deployment`, `remote`.
- `packages/local-web/`: Local React + TypeScript app entrypoint (Vite, Tailwind). Shell source in `packages/local-web/src`.
- `packages/remote-web/`: Remote deployment frontend entrypoint.
- `packages/web-core/`: Shared React + TypeScript frontend library used by local + remote web (`packages/web-core/src`).
- `shared/`: Generated TypeScript types (`shared/types.ts`, `shared/remote-types.ts`) and agent tool schemas (`shared/schemas/`). Do not edit generated files directly.
- `assets/`, `dev_assets_seed/`, `dev_assets/`: Packaged and local dev assets.
- `npx-cli/`: Files published to the npm CLI package.
- `scripts/`: Dev helpers (ports, DB preparation).
- `docs/`: Documentation files.

### Crate-specific guides
- [`crates/remote/AGENTS.md`](crates/remote/AGENTS.md) — Remote server architecture, ElectricSQL integration, mutation patterns, environment variables.
- [`docs/AGENTS.md`](docs/AGENTS.md) — Mintlify documentation writing guidelines and component reference.
- [`packages/local-web/AGENTS.md`](packages/local-web/AGENTS.md) — Web app design system styling guidelines.

## Managing Shared Types Between Rust and TypeScript

ts-rs allows you to derive TypeScript types from Rust structs/enums. By annotating your Rust types with #[derive(TS)] and related macros, ts-rs will generate .ts declaration files for those types.
When making changes to the types, you can regenerate them using `pnpm run generate-types`
Do not manually edit shared/types.ts, instead edit crates/server/src/bin/generate_types.rs

For remote/cloud types, regenerate using `pnpm run remote:generate-types`
Do not manually edit shared/remote-types.ts, instead edit crates/remote/src/bin/remote-generate-types.rs (see crates/remote/AGENTS.md for details).

## Build, Test, and Development Commands
- Install: `pnpm i`
- Run dev (web app + backend with ports auto-assigned): `pnpm run dev`
- Backend (watch): `pnpm run backend:dev:watch`
- Web app (dev): `pnpm run local-web:dev`
- Type checks: `pnpm run check` (frontend + all backend Rust workspaces) and `pnpm run backend:check` (all backend Rust workspaces, including `crates/remote`)
- Rust tests: `cargo test --workspace`
- Generate TS types from Rust: `pnpm run generate-types` (or `generate-types:check` in CI)
- Prepare SQLx (offline): `pnpm run prepare-db`
- Prepare SQLx (remote package, postgres): `pnpm run remote:prepare-db`
- Local NPX build: `pnpm run build:npx` then `pnpm pack` in `npx-cli/`
- Format code: `pnpm run format` (runs `cargo fmt` for all backend Rust workspaces + web-core/web Prettier)
- Lint: `pnpm run lint` (runs web/ui ESLint + `cargo clippy` for all backend Rust workspaces)

## Before Completing a Task
- Run `pnpm run format` to format all Rust workspaces and web code.

## Coding Style & Naming Conventions
- Rust: `rustfmt` enforced (`rustfmt.toml`); group imports by crate; snake_case modules, PascalCase types.
- TypeScript/React: ESLint + Prettier (2 spaces, single quotes, 80 cols). PascalCase components, camelCase vars/functions, kebab-case file names where practical.
- Keep functions small, add `Debug`/`Serialize`/`Deserialize` where useful.

## Testing Guidelines
- Rust: prefer unit tests alongside code (`#[cfg(test)]`), run `cargo test --workspace`. Add tests for new logic and edge cases.
- Web app: ensure `pnpm run check` and `pnpm run lint` pass. If adding runtime logic, include lightweight tests (e.g., Vitest) in the same directory.

## Security & Config Tips
- Use `.env` for local overrides; never commit secrets. Key envs: `FRONTEND_PORT`, `BACKEND_PORT`, `HOST`
- Dev ports and assets are managed by `scripts/setup-dev-environment.js`.

## Go Rewrite Workflow (Rust → Go Migration)

### 最终目标
当前项目中不再包含任何 Rust 代码及相关配置资源，所有文档更新到符合最新的 Go 方案。

### 任务执行流程（每个任务必须严格遵循）
1. **跑通测试** — 每个任务完成后必须跑通相关测试，作为初步完成的标志。
2. **全项目评估与审查** — 初步完成后，对全项目进行评估和审查，确保当前计划执行符合预期。有问题则修复。
3. **创建 Git 提交** — 审查无误后创建 git commit。
4. **决策反馈** — 如果有问题需要用户决策，则反馈给用户等待指示。
5. **自动继续** — 任务完成且无问题，则自动开始下一个任务，不等待用户确认。

### Go 项目结构
- `internal/domain/` — 领域模型（对应 Rust `crates/db/src/models/`）
- `internal/repository/` — 数据访问层（对应 Rust `crates/db/`）
- `internal/database/` — 数据库连接与迁移（对应 Rust `crates/db/` migrations）
- `internal/git/` — Git 操作服务（对应 Rust `crates/git/`）
- `internal/service/` — 业务逻辑服务层（对应 Rust `crates/services/`）
- `internal/handler/` — API Handlers（对应 Rust `crates/server/src/routes/`）
- `internal/executor/` — Claude Code 执行器（对应 Rust `crates/executors/`）
- `cmd/` — 应用入口
- `web/` — 前端资源（复用 TypeScript/React）

### Go 开发命令
- 构建: `go build ./...`
- 测试: `go test -race ./...`
- Vet: `go vet ./...`
- 单包测试: `go test -race -v ./internal/xxx/...`


