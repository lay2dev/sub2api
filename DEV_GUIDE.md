# Development Guide

This guide records repository-level development conventions for `sub2api`.

For harness and prompt architecture, see:
- `docs/HARNESS_ENGINEERING.md`

For the current Kimi Code integration plan, see:
- `plans/2026-03-20-kimi-code-support.md`

## Project basics

| Item | Value |
|------|-------|
| Upstream repo | `Wei-Shaw/sub2api` |
| This fork | `lay2dev/sub2api` |
| Backend | Go + Gin + Ent |
| Frontend | Vue 3 + Vite + pnpm |
| Database | PostgreSQL |
| Cache | Redis |

## Core repo conventions

### Package management

- Frontend uses `pnpm`, not `npm`.
- If `package.json` changes, commit the lockfile update too.

### Code generation

When editing Ent schema:

```bash
cd backend
go generate ./ent
go generate ./cmd/server
```

Commit generated files together with schema changes.

### Testing

Backend:

```bash
cd backend
go test -tags=unit ./...
go test -tags=integration ./...
```

Optional forked-RPC coverage for the USDC deposit watcher:

```bash
cd backend
BSC_FORK_RPC_URL=http://127.0.0.1:8545 \
ARBITRUM_FORK_RPC_URL=http://127.0.0.1:8546 \
BASE_FORK_RPC_URL=http://127.0.0.1:8547 \
go test -tags=integration ./internal/service -run 'TestUSDCDepositWatcherForkRPC_'
```

You can set any subset of these env vars. Missing chain env vars are skipped and do not fail the test run.

Frontend:

```bash
cd frontend
pnpm install
pnpm build
```

Run the narrowest relevant tests first, then broaden if needed.

### Linting

```bash
cd backend
golangci-lint run ./...
```

## Change discipline

- Keep changes surgical unless a broader refactor is explicitly requested.
- Do not revert unrelated local changes.
- Treat prompt assets and embedded instruction files as product code.
- Document behavior changes when modifying gateway routing, prompt assets, auth flows, or provider compatibility layers.

## Provider / gateway work

When touching provider integrations:
- document transport assumptions
- keep auth/header logic explicit
- avoid hiding provider quirks in unrelated business logic
- prefer configuration-driven compatibility layers before introducing full new platforms

This is especially important for coding-agent style upstreams such as Kimi Code.

## Recommended local workflow

```bash
# clone
 git clone <repo-url>

# frontend deps
cd frontend && pnpm install

# backend tests
cd ../backend && go test -tags=unit ./...

# run backend
cd ../backend && go run ./cmd/server
```

## What does not belong here

This file should not store:
- one developer's personal machine paths
- OS-specific password reset recipes
- private credentials
- fork-specific notes that are better kept in task docs

Put task-specific implementation plans in dedicated files instead.
