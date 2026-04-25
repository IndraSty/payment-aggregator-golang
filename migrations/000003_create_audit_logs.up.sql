CREATE TABLE transaction_audit_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id  UUID NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
    from_status     transaction_status,             -- NULL means initial creation
    to_status       transaction_status NOT NULL,
    source          VARCHAR(50) NOT NULL,           -- 'api', 'webhook', 'reconciliation', 'system'
    raw_payload     JSONB,                          -- original webhook/api payload for debugging
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Audit log is append-only: no UPDATE or DELETE allowed
CREATE RULE no_update_audit_logs AS ON UPDATE TO transaction_audit_logs DO INSTEAD NOTHING;
CREATE RULE no_delete_audit_logs AS ON DELETE TO transaction_audit_logs DO INSTEAD NOTHING;

CREATE INDEX idx_audit_logs_transaction_id ON transaction_audit_logs(transaction_id);
CREATE INDEX idx_audit_logs_created_at ON transaction_audit_logs(created_at DESC);