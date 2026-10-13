CREATE TABLE event_attachment_deliveries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_delivery_id BIGINT NOT NULL UNIQUE REFERENCES event_deliveries(id),
    request_body BYTEA,
    availability TEXT CHECK (availability IN ('available', 'not_provided', 'unavailable')),
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sending', 'delivered', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until TIMESTAMPTZ,
    last_http_status INTEGER,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);
CREATE INDEX event_attachment_deliveries_pending ON event_attachment_deliveries (next_attempt_at, id) WHERE status IN ('pending', 'sending');
