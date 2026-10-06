ALTER TABLE event_delivery_routes DROP COLUMN enabled_from;
CREATE TABLE event_delivery_backfills (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    terminal_serial_number TEXT NOT NULL REFERENCES terminals(serial_number),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL CHECK (ends_at > starts_at),
    preview_sha256 TEXT NOT NULL,
    queued_count INTEGER NOT NULL CHECK (queued_count > 0),
    created_by BIGINT NOT NULL REFERENCES admin_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE event_deliveries ALTER COLUMN device_event_id DROP NOT NULL;
ALTER TABLE event_deliveries ADD COLUMN retained_event_id BIGINT UNIQUE REFERENCES retained_access_events(id);
ALTER TABLE event_deliveries ADD COLUMN backfill_id BIGINT REFERENCES event_delivery_backfills(id);
ALTER TABLE event_deliveries ADD CONSTRAINT event_deliveries_source_check CHECK (num_nonnulls(device_event_id, retained_event_id) = 1);
