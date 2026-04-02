-- Add multi-use counters for redeem codes.
ALTER TABLE redeem_codes
ADD COLUMN IF NOT EXISTS max_uses INT NOT NULL DEFAULT 1;

ALTER TABLE redeem_codes
ADD COLUMN IF NOT EXISTS used_count INT NOT NULL DEFAULT 0;

-- Backfill to preserve existing single-use semantics.
UPDATE redeem_codes
SET
    max_uses = 1,
    used_count = CASE WHEN status = 'used' THEN 1 ELSE 0 END
WHERE
    max_uses IS DISTINCT FROM 1
    OR used_count IS DISTINCT FROM CASE WHEN status = 'used' THEN 1 ELSE 0 END;

-- Per-user redeem usage tracking for trial API key issuance.
CREATE TABLE IF NOT EXISTS redeem_code_usages (
    id BIGSERIAL PRIMARY KEY,
    redeem_code_id BIGINT NOT NULL REFERENCES redeem_codes(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(redeem_code_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_redeem_code_usages_redeem_code_id
ON redeem_code_usages (redeem_code_id);

CREATE INDEX IF NOT EXISTS idx_redeem_code_usages_user_id
ON redeem_code_usages (user_id);

CREATE INDEX IF NOT EXISTS idx_redeem_code_usages_api_key_id
ON redeem_code_usages (api_key_id);

CREATE INDEX IF NOT EXISTS idx_redeem_code_usages_used_at
ON redeem_code_usages (used_at);
