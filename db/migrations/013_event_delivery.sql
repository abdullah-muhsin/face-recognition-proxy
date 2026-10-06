CREATE TABLE event_delivery_routes (
    terminal_serial_number TEXT PRIMARY KEY REFERENCES terminals(serial_number),
    endpoint_url TEXT NOT NULL,
    signing_key_id TEXT NOT NULL,
    enabled_from TIMESTAMPTZ NOT NULL,
    enabled BOOLEAN NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by BIGINT NOT NULL REFERENCES admin_users(id)
);
CREATE TABLE event_deliveries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    device_event_id BIGINT NOT NULL UNIQUE REFERENCES device_events(id),
    terminal_serial_number TEXT NOT NULL REFERENCES terminals(serial_number),
    endpoint_url TEXT NOT NULL,
    signing_key_id TEXT NOT NULL,
    request_body BYTEA NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sending', 'delivered', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until TIMESTAMPTZ,
    last_http_status INTEGER,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);
CREATE INDEX event_deliveries_pending ON event_deliveries (next_attempt_at, id) WHERE status IN ('pending', 'sending');
CREATE INDEX event_deliveries_terminal ON event_deliveries (terminal_serial_number, id DESC);
