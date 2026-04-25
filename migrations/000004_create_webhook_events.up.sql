CREATE TABLE webhook_events (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider    payment_provider NOT NULL,
    event_type  VARCHAR(100) NOT NULL,
    raw_payload JSONB NOT NULL,
    processed   BOOLEAN NOT NULL DEFAULT FALSE,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_events_provider ON webhook_events(provider);
CREATE INDEX idx_webhook_events_processed ON webhook_events(processed);
CREATE INDEX idx_webhook_events_received_at ON webhook_events(received_at DESC);

-- Composite index for replay attack prevention
-- Quickly check if an event_id from a provider already exists
CREATE INDEX idx_webhook_events_provider_event ON webhook_events(provider, event_type, received_at);