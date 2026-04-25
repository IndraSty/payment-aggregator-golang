CREATE TYPE transaction_status AS ENUM (
    'pending',
    'success',
    'failed',
    'expired'
);

CREATE TYPE payment_provider AS ENUM (
    'midtrans',
    'xendit',
    'stripe'
);

CREATE TABLE transactions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    external_id     VARCHAR(255) NOT NULL,          -- ID from payment provider
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,   -- from X-Idempotency-Key header
    provider        payment_provider NOT NULL,
    amount          BIGINT NOT NULL CHECK (amount > 0),  -- in smallest currency unit (cents/rupiah)
    currency        VARCHAR(3) NOT NULL,            -- ISO 4217: IDR, USD, EUR
    status          transaction_status NOT NULL DEFAULT 'pending',
    payment_method  VARCHAR(50) NOT NULL,           -- e.g. credit_card, bank_transfer, gopay
    metadata        JSONB,                          -- provider-specific data
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expired_at      TIMESTAMPTZ
);

CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_external_id ON transactions(external_id);
CREATE INDEX idx_transactions_idempotency_key ON transactions(idempotency_key);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_provider ON transactions(provider);
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);

-- Auto-update updated_at on every row change
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();