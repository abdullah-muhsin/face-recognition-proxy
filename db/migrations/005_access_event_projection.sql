CREATE TABLE access_event_projections (
    device_event_id BIGINT PRIMARY KEY REFERENCES device_events(id) ON DELETE CASCADE,
    schema_version SMALLINT NOT NULL CHECK (schema_version = 1),
    classification_status TEXT NOT NULL CHECK (classification_status IN ('classified', 'unclassified')),
    major_event_type SMALLINT,
    sub_event_type INTEGER,
    event_description TEXT,
    occurred_at TIMESTAMPTZ,
    employee_number TEXT,
    employee_name TEXT,
    card_number TEXT,
    card_reader_number INTEGER,
    door_number INTEGER,
    source_ip_address TEXT,
    CHECK (
        (classification_status = 'classified' AND major_event_type IS NOT NULL AND sub_event_type IS NOT NULL)
        OR
        (classification_status = 'unclassified' AND major_event_type IS NULL AND sub_event_type IS NULL
         AND event_description IS NULL AND occurred_at IS NULL AND employee_number IS NULL
         AND employee_name IS NULL AND card_number IS NULL AND card_reader_number IS NULL
         AND door_number IS NULL AND source_ip_address IS NULL)
    )
);

CREATE INDEX access_event_projections_category_subtype_idx
    ON access_event_projections (major_event_type, sub_event_type, device_event_id DESC)
    WHERE classification_status = 'classified';
