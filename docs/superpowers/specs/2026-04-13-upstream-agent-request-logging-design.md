# Upstream Agent Request Logging Design

## Summary

Add a dedicated indexed system-log event for upstream agent requests sent by the OpenAI gateway so operators can:

- inspect the outbound upstream request payload
- search the payload from the existing `sub2api` frontend system-log view
- correlate the request with `request_id`, `client_request_id`, account, platform, and model

This change must stay inside the gateway/request-building observability boundary. It must not move provider transport quirks into unrelated business logic, usage logs, or repository-layer abstractions.

## User Goal

When `sub2api` forwards an agent-style request to an upstream provider, the operator should be able to:

1. see that outbound request in the system logs
2. search for keywords from the outbound request body in the frontend system-log page
3. inspect the request together with routing metadata such as account, upstream URL, request IDs, and model

## Scope

In scope:

- OpenAI gateway outbound HTTP request logging for upstream agent requests
- indexed system-log storage using the existing ops system-log pipeline
- frontend system-log detail rendering improvements for the new fields
- unit coverage for backend logging behavior and frontend rendering

Out of scope:

- usage-log schema changes
- request-details API/schema changes
- new full-text search infrastructure
- websocket payload capture
- provider-specific logging branches outside the gateway path touched by this feature

## Constraints

- Keep provider-specific transport behavior in gateway/request-building code.
- Reuse the existing indexed system-log pipeline and search behavior.
- Treat request payload logging as product code and keep sanitization explicit.
- Preserve current system-log query semantics; frontend search should continue to use the existing `q` filter.

## Approaches Considered

### Option A: Dedicated system-log event at the gateway boundary

Emit one indexed system-log event immediately before `httpUpstream.Do(...)` in the OpenAI gateway flow, with outbound request metadata and the sanitized upstream request body in `extra`.

Pros:

- smallest end-to-end change
- cleanest architecture boundary
- works with current system-log indexing and frontend search
- easy to correlate with existing request IDs

Cons:

- system-log storage volume increases because full request bodies become searchable

### Option B: Persist outbound request body into usage logs or request details

Pros:

- closer to request-history views

Cons:

- mixes transport observability into business/request reporting layers
- likely requires schema or API expansion
- broader blast radius than needed

### Option C: Reuse only failure-path upstream request capture

Pros:

- least code

Cons:

- only covers failed requests
- does not satisfy the requirement to inspect normal upstream agent requests

## Recommended Design

Use Option A.

Add a dedicated system-log event emitted by the OpenAI gateway immediately before the outbound upstream HTTP request is sent. The event will be indexed by the existing ops system-log sink and made searchable through the existing `q` filter because that query already matches `extra::text`.

## Architecture

### Backend emission point

Add a small helper in the OpenAI gateway service for outbound request logging. The helper is called from the HTTP forwarding path after the upstream request has been built and before `httpUpstream.Do(...)` executes.

The helper is responsible for:

- extracting correlation metadata from `gin.Context`
- serializing the outbound payload
- sanitizing the payload with the existing log-redaction utilities
- emitting a dedicated sink event with a stable message name and component

The repository layer and generic upstream HTTP client remain unchanged. They should not gain feature-specific logging knowledge.

### Indexed system-log storage

No schema change is required.

The existing ops system-log query already supports:

- exact filters on request/account/platform/model
- `q` matching against `message`, `request_id`, `client_request_id`, and `extra::text`

The new event will store the outbound request body in `extra.upstream_request_body`, which makes it searchable from the existing frontend system-log search box.

### Frontend rendering

Extend the system-log detail formatter to surface the new outbound-request fields so operators can read them directly in the system-log table detail column without adding a new page or API.

## Event Shape

Proposed message name:

- `openai.upstream_agent_request`

Proposed component:

- `service.openai_gateway`

Proposed indexed metadata:

- `request_id`
- `client_request_id`
- `account_id`
- `platform`
- `model`

Proposed `extra` fields:

- `account_name`
- `upstream_url`
- `upstream_path`
- `method`
- `stream`
- `openai_passthrough`
- `upstream_request_body`
- `upstream_request_body_truncated`

## Request Body Policy

The user requested full request-body logging. To satisfy that requirement while keeping the system operational, the gateway will store the full sanitized outbound request body unless it exceeds a fixed protection limit.

Protection rules:

1. sanitize with existing `logredact` helpers before writing
2. preserve the full sanitized body when it is within the configured limit
3. if the sanitized body exceeds the limit, store the truncated prefix and set `upstream_request_body_truncated=true`

Recommended limit:

- 128 KiB per event

Rationale:

- high enough for normal agent requests
- prevents a single large prompt or tool payload from bloating ops storage and frontend rendering

## Data Flow

1. The gateway builds the upstream request body and target URL.
2. The gateway logs one indexed sink event with sanitized outbound request metadata and body.
3. The gateway sends the request upstream.
4. The existing ops system-log sink persists the event.
5. The frontend system-log page loads the event through the existing API.
6. Operators search via `q`, which matches `extra::text`, including `upstream_request_body`.

## Error Handling

- Logging failure must not block the upstream request.
- If payload sanitization or JSON compaction fails, fall back to sanitized text form.
- If request body capture is empty, still emit the event with routing metadata so correlation remains available.
- If payload truncation occurs, set `upstream_request_body_truncated=true` so operators know the body shown is partial.

## Acceptance Mapping

### AC1: outbound upstream agent request is printed into logs

Implementation:

- emit `openai.upstream_agent_request` before outbound HTTP send

Validation:

- backend test asserts the sink receives the new event with request identifiers and routing metadata

### AC2: request body can be searched in the frontend system-log page

Implementation:

- store sanitized body in `extra.upstream_request_body`
- rely on existing `q` matching against `extra::text`

Validation:

- backend test asserts the payload lands in indexed `extra`
- frontend test asserts the detail renderer exposes the body text

### AC3: operators can inspect routing context together with the request

Implementation:

- include upstream URL/path, account, platform, model, and passthrough flag in `extra`

Validation:

- backend test checks the emitted event fields
- frontend test checks rendered detail text

## Test Plan

Backend:

- add unit coverage for the new logging helper
- add/extend a forwarding-path test to verify a log event is emitted before upstream send
- verify sensitive fields are redacted
- verify truncation flag behavior when the body exceeds the limit
- verify the new message is accepted by the ops system-log sink indexing rules

Frontend:

- extend `systemLogDetail` tests to include `upstream_url`, `upstream_path`, and `upstream_request_body`

Repository validation:

- run `bash scripts/validate.sh`

## Minimal File Touches

Expected primary files:

- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/ops_system_log_sink.go`
- `backend/internal/service/ops_system_log_sink_test.go`
- `frontend/src/views/admin/ops/utils/systemLogDetail.ts`
- `frontend/src/views/admin/ops/utils/__tests__/systemLogDetail.spec.ts`

Additional narrow helper/test files may be added only if they reduce coupling.

## Open Questions Resolved

- Full request body logging: yes, with sanitization and a hard size cap
- New search API: no
- New database schema: no
- New frontend page: no

## Implementation Boundary

The implementation should remain a surgical observability change:

- gateway emits the event
- existing ops system-log pipeline stores it
- existing frontend system-log page renders and searches it

No unrelated refactor is required.
