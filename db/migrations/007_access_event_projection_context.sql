ALTER TABLE access_event_projections
    ADD COLUMN event_state TEXT,
    ADD COLUMN source_mac_address TEXT,
    ADD COLUMN channel_id INTEGER,
    ADD COLUMN active_post_count INTEGER,
    ADD COLUMN short_serial_number TEXT,
    ADD COLUMN device_name TEXT,
    ADD COLUMN event_serial_number BIGINT,
    ADD COLUMN front_serial_number BIGINT,
    ADD COLUMN user_type TEXT,
    ADD COLUMN current_verify_mode TEXT,
    ADD COLUMN current_event BOOLEAN,
    ADD COLUMN mask TEXT,
    ADD COLUMN pictures_number INTEGER,
    ADD COLUMN pure_pwd_verify_enable BOOLEAN,
    ADD COLUMN face_rect_height TEXT,
    ADD COLUMN face_rect_width TEXT,
    ADD COLUMN face_rect_x TEXT,
    ADD COLUMN face_rect_y TEXT;

ALTER TABLE access_event_projections
    ADD CONSTRAINT access_event_projections_context_check
    CHECK (
        classification_status = 'classified'
        OR (
            event_state IS NULL
            AND source_mac_address IS NULL
            AND channel_id IS NULL
            AND active_post_count IS NULL
            AND short_serial_number IS NULL
            AND device_name IS NULL
            AND event_serial_number IS NULL
            AND front_serial_number IS NULL
            AND user_type IS NULL
            AND current_verify_mode IS NULL
            AND current_event IS NULL
            AND mask IS NULL
            AND pictures_number IS NULL
            AND pure_pwd_verify_enable IS NULL
            AND face_rect_height IS NULL
            AND face_rect_width IS NULL
            AND face_rect_x IS NULL
            AND face_rect_y IS NULL
        )
    );
