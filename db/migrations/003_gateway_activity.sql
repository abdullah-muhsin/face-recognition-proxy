CREATE TABLE gateway_activities (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    kind TEXT NOT NULL CHECK (kind ~ '^[a-z][a-z0-9_]*([.][a-z][a-z0-9_]*)*$'),
    terminal_serial_number TEXT REFERENCES terminals(serial_number),
    message TEXT NOT NULL CHECK (length(message) > 0),
    fields JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX gateway_activities_occurred_at_id_idx
    ON gateway_activities (occurred_at DESC, id DESC);
CREATE INDEX gateway_activities_terminal_occurred_at_id_idx
    ON gateway_activities (terminal_serial_number, occurred_at DESC, id DESC);
