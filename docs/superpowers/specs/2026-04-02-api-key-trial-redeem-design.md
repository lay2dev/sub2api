# API Key Trial Redeem Design

## Goal

Add a new redeem-code capability for logged-in users: redeeming a fixed campaign code from the existing user redeem page issues a new API key with a `20 USD` quota and a `7 day` expiration.

## Scope

- In scope:
  - existing logged-in user redeem flow: `POST /api/v1/redeem`
  - existing admin redeem-code generation flow
  - fixed-rule multi-use redeem code with `100` total uses
  - one issued API key per successful redemption
  - user-facing success response that returns the issued API key once
  - user redeem history support for the new redeem type
- Out of scope:
  - registration-time invitation or promo flows
  - configurable quota, validity, or max-uses for this first pass
  - adding balance, concurrency, or subscription rewards to this new code type
  - a separate standalone "trial code" management system
  - showing API key plaintext in redeem history after the initial success response

## Current Constraints

- `redeem_code` currently models mostly one-time redemption with `status`, `used_by`, and `used_at`.
- `RedeemService.Redeem` currently marks a code as used before applying the reward, which works for single-use codes but not for shared multi-use codes.
- `RedeemService.GetUserHistory` currently reads history from `redeem_codes.used_by`, which cannot represent many users redeeming the same code.
- API key creation already exists in `APIKeyService`, but repository create-paths must participate in the redeem transaction for atomic issuance.

## Chosen Approach

### 1. Extend the existing redeem-code system instead of adding a new product surface

Reuse the current redeem-code admin and user entry points. Add one new redeem type:

- `api_key_trial`

This keeps the feature aligned with the current redeem page and admin redeem-code management instead of introducing a second parallel campaign-code system.

### 2. Make the new type fixed-rule and server-owned

The first pass does not expose per-code tuning for this type. Every `api_key_trial` redeem code follows the same rules:

- `max_uses = 100`
- issued API key quota = `20 USD`
- issued API key expiry = `7 days`
- issued API key has no group binding
- one user can redeem a given code at most once

Admin callers may choose the code string and how many codes to generate, but they do not configure the reward details for this type.

### 3. Keep `redeem_codes` as the definition table, add a usage table for multi-use history

Add fields to `redeem_codes`:

- `max_uses`
- `used_count`

Add a new `redeem_code_usages` table:

- `redeem_code_id`
- `user_id`
- `api_key_id`
- `used_at`

Required uniqueness:

- unique `(redeem_code_id, user_id)`

This separates:

- redeem-code definition and total usage accounting
- per-user redemption history
- linkage to the issued API key

### 4. Preserve old one-time redeem behavior

Legacy types continue to behave as single-use codes:

- `balance`
- `concurrency`
- `subscription`
- `invitation`

For those legacy types:

- generated codes default to `max_uses = 1`
- the existing single-use semantics remain
- `used_by` / `used_at` remain the authoritative fields

For `api_key_trial`:

- `used_by` / `used_at` are not authoritative and should remain unset on the main redeem-code row
- `used_count` and `redeem_code_usages` become the source of truth
- `status` remains `unused` while `used_count < max_uses`
- once `used_count == max_uses`, the code transitions to a non-usable terminal state (`used` is sufficient for the first pass)

## Data Model

### Redeem code type

Add:

- `RedeemTypeAPIKeyTrial = "api_key_trial"`

### Redeem code fields

Add to `redeem_codes`:

- `max_uses int`
- `used_count int`

Field semantics:

- legacy single-use codes use `max_uses = 1`
- `api_key_trial` uses `max_uses = 100`
- `used_count` is incremented once per successful user redemption

### Data migration

Backfill existing rows so current behavior is preserved after rollout:

- existing one-time redeem codes should get `max_uses = 1`
- existing rows with `status = used` should get `used_count = 1`
- existing rows with usable status should get `used_count = 0`

This avoids accidental behavior drift where old redeem codes become effectively unlimited because the new counters were left at zero defaults.

### Redeem code usage record

Add `redeem_code_usages` schema and repository mapping for:

- many users redeeming one code
- associating one redemption with one issued API key
- rendering user history without overloading `redeem_codes.used_by`

The usage row should record the issued API key id so operators can audit which key was created by which campaign redemption.

## Service Responsibilities

### RedeemService

`RedeemService` remains the orchestration layer for redeem business rules.

For legacy redeem types it keeps the current flow.

For `api_key_trial` it owns:

- loading the redeem code under row lock
- checking remaining uses
- checking whether the current user already redeemed the code
- issuing the API key
- writing the usage record
- incrementing `used_count`
- transitioning the code to terminal state when exhausted

### API key issuance

Do not hide API key creation inside handlers or directly inside unrelated repository code.

Preferred shape:

- inject a narrow API-key issuer dependency into `RedeemService`
- implement it with the existing `APIKeyService`

This keeps:

- business rules in service code
- API key generation/validation logic close to existing API key code
- handler and repository layers free of cross-cutting reward logic

### Transaction participation

The trial redeem path must be atomic:

1. usage eligibility passes
2. API key is created
3. usage row is written
4. `used_count` is incremented

If any step fails, none of the above should commit.

That requires API-key creation to honor the current transaction context. In practice, the API key repository create path must use `clientFromContext(...)` instead of always using the root Ent client.

## Redeem Flow

### User redeem endpoint

Continue using:

- `POST /api/v1/redeem`

Flow for `api_key_trial`:

1. authenticate the user through the existing JWT flow
2. load the redeem code by code string
3. if type is not `api_key_trial`, use the existing legacy branch
4. if type is `api_key_trial`, begin transaction
5. lock the redeem-code row
6. verify `status` is usable and `used_count < max_uses`
7. verify no usage row exists for `(redeem_code_id, user_id)`
8. create a new API key with:
   - generated random key
   - active status
   - `quota = 20`
   - `quota_used = 0`
   - `expires_at = now + 7 days`
   - `group_id = nil`
   - a deterministic display name such as `Trial Key YYYY-MM-DD`
9. insert `redeem_code_usage`
10. increment `used_count`
11. if the count reaches `max_uses`, mark the redeem code exhausted
12. commit transaction
13. return the redeem result plus the issued API key

## Response Shape

Extend the user-facing redeem success DTO with:

- `issued_api_key`

Recommended contents:

- `id`
- `key`
- `name`
- `status`
- `quota`
- `quota_used`
- `expires_at`

Behavior:

- only the immediate redeem success response returns API key plaintext
- history endpoints must not return key plaintext for past redemptions

This lets the frontend display a copy-once success card without turning history into a sensitive key-recovery surface.

## User History

The current history query based on `redeem_codes.used_by` is insufficient for shared codes.

For `api_key_trial`, user history should be sourced from `redeem_code_usages` and projected into the existing redeem-history DTO shape:

- code metadata comes from `redeem_codes`
- `used_at` comes from the usage row for the current user
- no plaintext API key is returned in history

The history UI may label this type as a trial API key redemption and show:

- redeem code
- redemption time
- quota (`20 USD`)
- optional non-sensitive metadata derived from the issued key, such as expiry time, without returning the plaintext key

## Admin Behavior

### Generation

Extend the existing admin generate endpoint to accept:

- `api_key_trial`

For this type the service should normalize the input:

- ignore caller-provided reward `value`
- set `max_uses = 100`
- set `used_count = 0`
- keep code status usable at creation

Admins can still generate multiple campaign codes in one request. Each generated code is separately redeemable up to `100` times.

### Listing

Admin redeem-code list output should expose enough data to operate the campaign:

- type
- `used_count`
- `max_uses`
- remaining uses

### Deletion

Delete behavior should not permit deleting a partially used trial code.

For `api_key_trial`, any code with `used_count > 0` should be treated as already consumed for delete-guard purposes, even if it is not yet exhausted.

### Create-and-redeem endpoint

The existing admin "create and redeem for one target user" endpoint is not required for this feature and does not need first-pass support for `api_key_trial`.

## Error Handling

Add explicit business errors for:

- trial redeem code exhausted
- user already redeemed this trial code

Behavior expectations:

- duplicate redemption by the same user returns conflict
- the `101st` redemption returns an exhausted/max-used style error
- API key creation failure rolls back the entire transaction
- legacy redeem types keep their current error behavior

## Testing Strategy

### Backend

Add focused tests for:

- successful `api_key_trial` redemption creates exactly one API key
- issued API key has `quota = 20`
- issued API key has `quota_used = 0`
- issued API key expiration is approximately `now + 7 days`
- same user cannot redeem the same code twice
- different users can redeem the same code until the `100th` success
- the `101st` attempt fails with the expected error
- transaction rollback when API key creation fails leaves:
  - no API key
  - no usage row
  - unchanged `used_count`
- user history includes trial redemptions without returning plaintext key material
- legacy redeem types still pass their current tests

### Frontend

Add/adjust tests for:

- redeem success card renders `issued_api_key` when present
- copy action is available for the one-time shown key
- non-trial redeem success rendering remains unchanged

### Full validation

Run:

- `bash scripts/validate.sh`

## Docs Impact

Update stable docs or admin-facing descriptions to note:

- a new `api_key_trial` redeem type exists
- it is redeemed from the normal logged-in redeem page
- each redemption issues one API key with `20 USD` quota and `7 day` expiry
- one code can be redeemed by up to `100` different users
