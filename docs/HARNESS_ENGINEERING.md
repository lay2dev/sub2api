# Harness Engineering

This document defines the repository-level harness design for `sub2api`.

It exists to make prompt behavior, tool behavior, approval behavior, and provider-specific instruction wiring explicit and auditable.

## Goals

- Keep harness behavior deterministic enough to debug.
- Separate generic coding-agent policy from provider-specific packaging.
- Avoid scattering prompt rules across handlers and services without documentation.
- Make future provider integrations (Codex, OpenCode, Kimi Code, Claude Code-like upstreams) easier to reason about.

## Current architecture

### Canonical instruction source

The main embedded instruction bundle for OpenAI/Codex-style clients is:

- `backend/internal/pkg/openai/instructions.txt`

It is compiled into the backend via:

- `backend/internal/pkg/openai/constants.go`

This means instruction changes are product behavior changes, not just docs changes.

### Adapter / bridge prompt

A second prompt layer exists for OpenCode compatibility:

- `backend/internal/service/prompts/codex_opencode_bridge.txt`

This file translates Codex-oriented expectations into OpenCode tool semantics.

### Implication

The project already has a two-layer harness structure:

1. base agent instruction set
2. runtime/provider bridge layer

That is the right pattern and should be made explicit for all future harness integrations.

## Harness design rules

### 1. Single canonical base prompt per provider family

For each provider family, maintain one canonical base instruction source.

Examples:
- OpenAI / Codex family -> `backend/internal/pkg/openai/instructions.txt`
- Additional families should follow the same pattern under a clearly named package or prompt directory.

Do not duplicate the same base behavior in multiple handlers or services.

### 2. Bridge prompts are adapters, not replacements

Bridge prompts should only do one of these:
- tool-name translation
- environment capability explanation
- sandbox/approval translation
- provider-specific request/response caveats

They should not silently redefine core product behavior that belongs in the base instructions.

### 3. Prompt assets are versioned product code

Treat these files as code:
- `backend/internal/pkg/openai/instructions.txt`
- `backend/internal/service/prompts/codex_opencode_bridge.txt`
- any future harness prompt assets

Required discipline:
- document why behavior changed
- keep edits small and reviewable
- prefer additive, testable changes over broad rewrites

### 4. Provider fingerprinting must be explicit

If a provider requires client fingerprinting to unlock functionality, document it as harness behavior.

For example, Kimi Code requires a coding-agent fingerprint for `/chat/completions` to succeed. This is not prompt semantics; it is harness transport behavior.

That kind of requirement belongs in provider integration docs and request builders, not hidden in random business logic.

### 5. Request-shape requirements belong beside transport code

If a provider is sensitive to:
- headers
- auth style
- SSE format
- tool-call chunk shape
- reasoning fields

then those rules must be documented near the gateway/request-builder code and referenced from docs.

## Practical standards for this repo

### Base instruction files

- Keep provider-family instruction files under stable, discoverable paths.
- Prefer one canonical file per family.
- If embedded into binaries, call that out in docs and comments.

### Prompt layering

Use this order conceptually:

1. product/system instructions
2. provider-family base instructions
3. runtime bridge prompt
4. per-request dynamic context

Avoid hidden prompt concatenation logic spread across unrelated packages.

### Naming

Use filenames that describe role, not temporary implementation details.

Good:
- `instructions.txt`
- `codex_opencode_bridge.txt`
- `kimi_code_gateway_notes.md`

Bad:
- `prompt_new.txt`
- `temp_instructions.txt`
- `final_prompt_v2.txt`

### Change management

When changing harness behavior, capture:
- what changed
- why it changed
- which provider/runtime it affects
- how to validate it

For meaningful behavior changes, add or update tests where the repo already has coverage hooks.

## Kimi Code notes

Kimi Code should be treated as an OpenAI-compatible transport with provider-specific harness constraints.

Confirmed integration facts:
- base URL: `https://api.kimi.com/coding/v1`
- chat endpoint: `POST /chat/completions`
- models endpoint: `GET /models`
- reliable auth: `Authorization: Bearer <token>`
- required coding-agent fingerprint: `User-Agent: claude-code/1.0`
- streaming: OpenAI-style SSE with `reasoning_content` and incremental tool call arguments

This means first-pass support should live in request-building and provider config, not in a special prompt prefix hack.

## Recommended next cleanup steps

### Near term

- Keep `backend/internal/pkg/openai/instructions.txt` as the canonical Codex-family base prompt.
- Keep `backend/internal/service/prompts/codex_opencode_bridge.txt` as an explicit adapter layer.
- Add provider integration notes when upstreams need non-obvious transport fingerprints.
- Keep project docs free of machine-specific local setup unless clearly marked as local-only.

### Medium term

- Create a small docs index for prompt assets and where they are injected.
- Add tests around prompt assembly and provider request shaping where practical.
- Introduce a consistent pattern for provider-specific default headers and auth styles.

## Non-goals

This document does not try to define:
- model behavior policy for every provider
- pricing logic
- admin UI behavior
- full prompt text contents

It defines where harness behavior should live and how to keep it maintainable.
