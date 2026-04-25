CREATE TYPE reconciliation_status AS ENUM (
    'running',
    'completed',
    'failed'
);

CREATE TABLE reconciliation_reports (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider        payment_provider NOT NULL,
    date            DATE NOT NULL,
    total_checked   INTEGER NOT NULL DEFAULT 0,
    discrepancies   INTEGER NOT NULL DEFAULT 0,
    status          reconciliation_status NOT NULL DEFAULT 'running',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reconciliation_provider ON reconciliation_reports(provider);
CREATE INDEX idx_reconciliation_date ON reconciliation_reports(date DESC);
CREATE INDEX idx_reconciliation_status ON reconciliation_reports(status);

-- Prevent duplicate reconciliation for same provider+date
CREATE UNIQUE INDEX idx_reconciliation_provider_date 
    ON reconciliation_reports(provider, date) 
    WHERE status != 'failed';