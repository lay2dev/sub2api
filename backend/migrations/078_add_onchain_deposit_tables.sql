-- Migration: add onchain USDC deposit scan state and deposit ledger tables

CREATE TABLE IF NOT EXISTS onchain_deposit_scan_states (
    id BIGSERIAL PRIMARY KEY,
    chain VARCHAR(32) NOT NULL,
    last_scanned_block BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_onchain_deposit_scan_states_chain
ON onchain_deposit_scan_states (chain);

CREATE TABLE IF NOT EXISTS onchain_deposits (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chain VARCHAR(32) NOT NULL,
    token_symbol VARCHAR(16) NOT NULL,
    token_contract VARCHAR(42) NOT NULL,
    tx_hash VARCHAR(66) NOT NULL,
    log_index BIGINT NOT NULL,
    block_number BIGINT NOT NULL,
    block_hash VARCHAR(66) NOT NULL,
    from_address VARCHAR(42) NOT NULL,
    to_address VARCHAR(42) NOT NULL,
    amount_raw VARCHAR(80) NOT NULL,
    amount_credit DECIMAL(20,8) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'detected',
    credited_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_onchain_deposits_chain_tx_log
ON onchain_deposits (chain, tx_hash, log_index);

CREATE INDEX IF NOT EXISTS idx_onchain_deposits_chain_status
ON onchain_deposits (chain, status);

CREATE INDEX IF NOT EXISTS idx_onchain_deposits_user_id
ON onchain_deposits (user_id);

COMMENT ON TABLE onchain_deposit_scan_states IS 'Per-chain scan cursor for onchain USDC deposit watcher';
COMMENT ON TABLE onchain_deposits IS 'Detected and credited onchain USDC deposits for user binding addresses';
