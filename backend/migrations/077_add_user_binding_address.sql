-- Migration: add deterministic EVM binding address to users

ALTER TABLE users
ADD COLUMN IF NOT EXISTS binding_address VARCHAR(42) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_binding_address_unique
ON users(binding_address)
WHERE deleted_at IS NULL AND binding_address <> '';

COMMENT ON COLUMN users.binding_address IS 'Deterministic EVM binding address derived from configured mnemonic and user ID';
