# Debugging

This document captures the minimum workflow for debugging local changes.

## Start the backend

```bash
cd backend
go run ./cmd/server
```

## Build the frontend

```bash
cd frontend
pnpm install
pnpm build
```

## Run validation

```bash
bash scripts/validate.sh
```

## Kimi Code integration debugging

When testing Kimi Code compatibility, verify these first:
- AC1: base URL is `https://api.kimi.com/coding/v1`
- AC2: auth/request shaping can include `User-Agent: claude-code/1.0`
- AC3: `/models` returns `kimi-for-coding`
- AC4: non-stream `/chat/completions` succeeds
- AC5: stream responses preserve SSE chunks and `[DONE]`
- AC6: the first pass stays OpenAI-compatible and does not require a full `PlatformKimi`

## Failure triage

Check in this order:
1. wrong base URL
2. wrong auth header or auth scheme
3. missing coding-agent `User-Agent`
4. request path mismatch (`/models` vs `/chat/completions`)
5. stream handling stripping provider-specific fields
