# Task

## Goal
Add minimal-change support for Kimi Code (overseas) to `sub2api` by reusing the existing OpenAI-compatible gateway path with explicit upstream configuration.

## Acceptance Criteria
AC1: sub2api can forward requests to `https://api.kimi.com/coding/v1` using configurable auth/header settings.
AC2: account configuration can inject `User-Agent: claude-code/1.0` for Kimi Code upstream requests.
AC3: Kimi Code `/models` can return the currently exposed model `kimi-for-coding`.
AC4: non-stream `chat/completions` requests can succeed against Kimi Code.
AC5: stream `chat/completions` requests can succeed and preserve SSE behavior.
AC6: the implementation does not require introducing a full new `PlatformKimi` on the first pass.

## Constraints
- Keep the first pass configuration-driven.
- Do not rely on prompt-prefix hacks as the primary compatibility mechanism.
- Keep model IDs unchanged when forwarding.
- Prefer surgical changes over broad refactors.

## Non-Goals
- Add a first-class dedicated Kimi platform in admin/UI on the first pass.
- Infer or rewrite `kimi-for-coding` to internal backend model names.
- Redesign the entire prompt assembly system.

## Validation
- `bash scripts/validate.sh`
- `bash scripts/smoke-kimi-code.sh`

## Notes
Key confirmed upstream behavior:
- base URL: `https://api.kimi.com/coding/v1`
- auth: `Authorization: Bearer <token>` is the safe default
- required coding-agent fingerprint: `User-Agent: claude-code/1.0`
- stream format: OpenAI-style SSE with `reasoning_content` and incremental tool call arguments
