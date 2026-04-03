# API Key Trial Admin Generation Design

## Goal

Allow Sub2API administrators to generate `api_key_trial` redeem codes from the existing admin redeem-code page, with each generated code using a real fixed 6-character code format and server-configured trial defaults for quota, max uses, and expiry.

## Scope

- In scope:
  - admin redeem-code generation UI
  - admin redeem-code generation backend behavior
  - real 6-character code generation for `api_key_trial`
  - deployment-level config defaults for trial quota, max uses, and expiry days
  - keeping existing redeem-code page and backend endpoints
- Out of scope:
  - user redeem flow changes
  - moving config into admin settings UI
  - changing code format for non-trial redeem types
  - introducing a separate admin page for trial code generation

## Current Problems

- Backend now supports `api_key_trial`, but the admin frontend still only offers:
  - `balance`
  - `concurrency`
  - `subscription`
  - `invitation`
- The backend trial generation path currently normalizes fixed values, but it still relies on the generic redeem-code generator rather than a guaranteed 6-character invitation-code format.
- Trial defaults are hardcoded (`20 USD`, `100 uses`, `7 days`) instead of being deployer-configurable.

## Chosen Approach

### 1. Reuse the existing admin redeem-code page

Do not add a new page. Extend the current admin redeem-code generation flow in:

- `frontend/src/views/admin/RedeemView.vue`
- `frontend/src/api/admin/redeem.ts`
- existing backend `/admin/redeem-codes/generate`

This keeps the feature aligned with current operator workflows and avoids duplicate management surfaces.

### 2. Treat `api_key_trial` as a first-class admin-visible redeem type

The admin UI should expose `api_key_trial` alongside the existing types.

When selected:

- do not show amount input
- do not show subscription group input
- do not show subscription validity input
- show a short explanatory hint describing the current trial policy

The count input remains available so admins can generate multiple trial codes in one action.

### 3. Make 6-character code generation type-specific

For `api_key_trial` only:

- generate codes as fixed 6-character uppercase alphanumeric strings
- retry on collisions until a unique code is created

For all existing types:

- keep the current generic redeem-code generation behavior unchanged

This avoids unintended product changes to legacy code formats while ensuring the activate-page UX and backend reality actually match.

### 4. Move trial defaults into backend config with safe fallbacks

Add deployment-level config fields under the existing config model, using defaults when omitted.

Recommended fields:

- `default.api_key_trial_quota_usd`
- `default.api_key_trial_max_uses`
- `default.api_key_trial_expires_in_days`

Default values:

- `20`
- `100`
- `7`

These values should be read only by the backend. The admin UI may describe them, but must not own or persist them.

## Backend Design

### Config

Add the three trial-default fields to backend config and defaults.

Behavior:

- if config is missing, fall back to `20 / 100 / 7`
- config values apply only to `api_key_trial`

### Admin generation logic

In `GenerateRedeemCodes`:

- if `input.Type == RedeemTypeAPIKeyTrial`
  - force `Value = 0`
  - force `MaxUses = cfg.Default.APIKeyTrialMaxUses`
  - force `UsedCount = 0`
  - force code generation to the 6-character generator

The generated code remains a normal redeem-code row, so the downstream redeem flow continues to work without any new table or endpoint shape.

### Code generator rules

`api_key_trial` code generation rules:

- charset: `A-Z0-9`
- length: `6`
- uppercase only

Uniqueness:

- use existence check and retry loop
- cap retries defensively to avoid infinite loops under collision storms

## Frontend Admin Design

### Generate dialog

Add `api_key_trial` to the admin type selector.

When selected:

- amount/value input hidden
- subscription-specific inputs hidden
- informational hint shown, for example:
  - 6-character code
  - each code can be redeemed `N` times
  - each redemption issues a key with `Q USD` quota for `D` days

The text may either:

- use static wording that references the backend-managed default policy
- or consume backend-exposed config in a later enhancement

For this first pass, static explanatory text is acceptable as long as the real backend behavior is authoritative.

### List table

Support display of the new type in:

- type badge
- filter dropdown

For the value column, `api_key_trial` should not show a misleading bare `0`.
Instead, render a human-readable label such as:

- `Trial API Key`
- or `20 USD / 7d key`

The exact wording can stay simple, but it should clearly distinguish trial codes from balance codes.

## Testing Strategy

### Backend

Add focused tests for:

- config defaults are applied when trial config is omitted
- admin generation of `api_key_trial` forces:
  - `value = 0`
  - `max_uses = configured default`
  - `used_count = 0`
- generated trial codes match the 6-character uppercase alphanumeric format
- legacy redeem types still use the existing generator path

### Frontend

Add focused tests for:

- admin generate form accepts `api_key_trial`
- selecting `api_key_trial` hides irrelevant inputs
- the request still submits count + type correctly
- filters and table display handle the new type without crashing

## File Changes

- `backend/internal/config/config.go`
  - add config fields and defaults
- `backend/internal/service/admin_service.go`
  - read config-backed defaults and route `api_key_trial` to a 6-character generator
- `backend/internal/handler/admin/redeem_handler.go`
  - keep accepting `api_key_trial`
- `frontend/src/types/index.ts`
  - add `api_key_trial` to `RedeemCodeType`
- `frontend/src/api/admin/redeem.ts`
  - no contract redesign needed, but trial type must flow through types cleanly
- `frontend/src/views/admin/RedeemView.vue`
  - add UI support for generating and listing `api_key_trial`

## Non-Goals

- Editing trial defaults from the admin settings page
- Sharing code-format configuration across all redeem-code types
- Changing the activate-page input format away from 6 characters
