CREATE TABLE terminals (
    serial_number TEXT PRIMARY KEY,
    pushsdk_serial TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    credential_fingerprint TEXT NOT NULL,
    security_version SMALLINT NOT NULL CONSTRAINT terminals_security_version_check
        CHECK (security_version IN (3, 4)),
    command_interval_seconds INTEGER NOT NULL CONSTRAINT terminals_command_interval_seconds_check
        CHECK (command_interval_seconds BETWEEN 1 AND 300),
    error_delay_seconds INTEGER NOT NULL CONSTRAINT terminals_error_delay_seconds_check
        CHECK (error_delay_seconds BETWEEN 1 AND 300),
    connection_status TEXT NOT NULL DEFAULT 'offline' CONSTRAINT terminals_connection_status_check
        CHECK (connection_status IN ('offline', 'authenticating', 'online', 'error')),
    last_seen_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE device_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    terminal_serial_number TEXT NOT NULL REFERENCES terminals(serial_number),
    vendor_event_id TEXT NOT NULL,
    data_format TEXT NOT NULL CONSTRAINT device_events_data_format_check
        CHECK (data_format IN ('jsonData', 'xmlData', 'boundaryData', 'noData')),
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload_base64 TEXT,
    UNIQUE (terminal_serial_number, vendor_event_id)
);

CREATE INDEX device_events_received_at_idx ON device_events (received_at DESC);
CREATE INDEX device_events_terminal_received_at_idx
    ON device_events (terminal_serial_number, received_at DESC);

CREATE TABLE admin_users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE admin_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_user_id BIGINT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX admin_sessions_expires_at_idx ON admin_sessions (expires_at);

CREATE TABLE gateway_activities (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    kind TEXT NOT NULL CONSTRAINT gateway_activities_kind_check
        CHECK (kind ~ '^[a-z][a-z0-9_]*([.][a-z][a-z0-9_]*)*$'),
    terminal_serial_number TEXT REFERENCES terminals(serial_number),
    message TEXT NOT NULL CONSTRAINT gateway_activities_message_check
        CHECK (length(message) > 0),
    fields JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX gateway_activities_occurred_at_id_idx
    ON gateway_activities (occurred_at DESC, id DESC);
CREATE INDEX gateway_activities_terminal_occurred_at_id_idx
    ON gateway_activities (terminal_serial_number, occurred_at DESC, id DESC);

CREATE TABLE pushsdk_sessions (
    terminal_serial_number TEXT PRIMARY KEY REFERENCES terminals(serial_number) ON DELETE CASCADE,
    configuration_fingerprint TEXT NOT NULL CONSTRAINT pushsdk_sessions_configuration_fingerprint_check
        CHECK (configuration_fingerprint ~ '^[0-9a-f]{64}$'),
    payload_mode TEXT NOT NULL CONSTRAINT pushsdk_sessions_payload_mode_check
        CHECK (payload_mode IN ('plaintext', 'encrypted')),
    salt TEXT NOT NULL CONSTRAINT pushsdk_sessions_salt_check
        CHECK (salt ~ '^[0-9A-Za-z]{64}$'),
    login_challenge TEXT NOT NULL CONSTRAINT pushsdk_sessions_login_challenge_check
        CHECK (login_challenge ~ '^[0-9A-Za-z]{64}$'),
    next_challenge TEXT NOT NULL DEFAULT '' CONSTRAINT pushsdk_sessions_next_challenge_check
        CHECK (next_challenge = '' OR next_challenge ~ '^[0-9a-f]{64}$'),
    iterations INTEGER NOT NULL CONSTRAINT pushsdk_sessions_iterations_check
        CHECK (iterations BETWEEN 500 AND 5000),
    created_at TIMESTAMPTZ NOT NULL,
    next_challenge_at TIMESTAMPTZ,
    authenticated BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT pushsdk_sessions_authenticated_state_check CHECK (
        (authenticated = FALSE AND next_challenge = '' AND next_challenge_at IS NULL)
        OR
        (authenticated = TRUE AND next_challenge ~ '^[0-9a-f]{64}$' AND next_challenge_at IS NOT NULL)
    )
);

CREATE TABLE access_event_projections (
    device_event_id BIGINT PRIMARY KEY REFERENCES device_events(id) ON DELETE CASCADE,
    schema_version SMALLINT NOT NULL CONSTRAINT access_event_projections_schema_version_check
        CHECK (schema_version > 0),
    classification_status TEXT NOT NULL CONSTRAINT access_event_projections_classification_status_check
        CHECK (classification_status IN ('classified', 'unclassified')),
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
    event_state TEXT,
    source_mac_address TEXT,
    channel_id INTEGER,
    active_post_count INTEGER,
    short_serial_number TEXT,
    device_name TEXT,
    event_serial_number BIGINT,
    front_serial_number BIGINT,
    user_type TEXT,
    current_verify_mode TEXT,
    current_event BOOLEAN,
    mask TEXT,
    pictures_number INTEGER,
    pure_pwd_verify_enable BOOLEAN,
    face_rect_height TEXT,
    face_rect_width TEXT,
    face_rect_x TEXT,
    face_rect_y TEXT,
    CONSTRAINT access_event_projections_identity_check CHECK (
        (classification_status = 'classified' AND major_event_type IS NOT NULL AND sub_event_type IS NOT NULL)
        OR
        (classification_status = 'unclassified' AND major_event_type IS NULL AND sub_event_type IS NULL
         AND event_description IS NULL AND occurred_at IS NULL AND employee_number IS NULL
         AND employee_name IS NULL AND card_number IS NULL AND card_reader_number IS NULL
         AND door_number IS NULL AND source_ip_address IS NULL)
    ),
    CONSTRAINT access_event_projections_context_check CHECK (
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
    )
);

CREATE INDEX access_event_projections_category_subtype_idx
    ON access_event_projections (major_event_type, sub_event_type, device_event_id DESC)
    WHERE classification_status = 'classified';

CREATE TABLE isapi_commands (
    uuid TEXT PRIMARY KEY CONSTRAINT isapi_commands_uuid_check
        CHECK (uuid ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'),
    terminal_serial_number TEXT NOT NULL REFERENCES terminals(serial_number),
    created_by_admin_user_id BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
    created_by_username TEXT NOT NULL CONSTRAINT isapi_commands_created_by_username_check
        CHECK (length(created_by_username) > 0),
    method TEXT NOT NULL CONSTRAINT isapi_commands_method_check
        CHECK (method IN ('GET', 'POST', 'PUT', 'DELETE')),
    url TEXT NOT NULL CONSTRAINT isapi_commands_url_check
        CHECK (length(url) BETWEEN 8 AND 4096 AND left(url, 7) = '/ISAPI/'),
    data_format TEXT NOT NULL CONSTRAINT isapi_commands_data_format_check
        CHECK (data_format IN ('jsonData', 'xmlData', 'boundaryData', 'noData')),
    request_data BYTEA NOT NULL,
    status TEXT NOT NULL CONSTRAINT isapi_commands_status_check
        CHECK (status IN ('queued', 'sent', 'completed', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    response_data_format TEXT CONSTRAINT isapi_commands_response_data_format_check
        CHECK (response_data_format IN ('jsonData', 'xmlData', 'boundaryData', 'noData')),
    response_data_format_declared BOOLEAN,
    response_data_base64 TEXT,
    CONSTRAINT isapi_commands_request_data_check CHECK (
        (data_format = 'noData' AND octet_length(request_data) = 0)
        OR (data_format <> 'noData' AND octet_length(request_data) > 0)
    ),
    CONSTRAINT isapi_commands_response_state_check CHECK ((
        (status = 'queued'
         AND sent_at IS NULL
         AND completed_at IS NULL
         AND response_data_format IS NULL
         AND response_data_format_declared IS NULL
         AND response_data_base64 IS NULL)
        OR
        (status = 'sent'
         AND sent_at IS NOT NULL
         AND completed_at IS NULL
         AND response_data_format IS NULL
         AND response_data_format_declared IS NULL
         AND response_data_base64 IS NULL)
        OR
        (status = 'completed'
         AND sent_at IS NOT NULL
         AND completed_at IS NOT NULL
         AND (
             (response_data_format IS NOT NULL
              AND response_data_format_declared = TRUE
              AND response_data_base64 IS NOT NULL)
             OR
             (data_format = 'noData'
              AND response_data_format IS NULL
              AND response_data_format_declared = FALSE
              AND response_data_base64 IS NOT NULL)
         ))
        OR
        (status = 'expired'
         AND sent_at IS NULL
         AND completed_at IS NOT NULL
         AND response_data_format IS NULL
         AND response_data_format_declared IS NULL
         AND response_data_base64 IS NULL)
    ) IS TRUE),
    CONSTRAINT isapi_commands_expiry_check CHECK (expires_at > created_at)
);

CREATE INDEX isapi_commands_terminal_created_idx
    ON isapi_commands (terminal_serial_number, created_at DESC, uuid DESC);
CREATE INDEX isapi_commands_terminal_status_created_idx
    ON isapi_commands (terminal_serial_number, status, created_at ASC, uuid ASC);

CREATE TABLE access_event_sync_runs (
    uuid TEXT PRIMARY KEY CONSTRAINT access_event_sync_runs_uuid_check
        CHECK (uuid ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'),
    terminal_serial_number TEXT NOT NULL REFERENCES terminals(serial_number),
    created_by_admin_user_id BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
    created_by_username TEXT NOT NULL CONSTRAINT access_event_sync_runs_created_by_username_check
        CHECK (length(created_by_username) > 0),
    search_id TEXT NOT NULL CONSTRAINT access_event_sync_runs_search_id_check
        CHECK (length(search_id) BETWEEN 1 AND 64),
    status TEXT NOT NULL CONSTRAINT access_event_sync_runs_status_check
        CHECK (status IN ('awaiting_time', 'running', 'completed', 'failed')),
    search_started_at TIMESTAMPTZ,
    search_ended_at TIMESTAMPTZ,
    total_matches INTEGER,
    pages_completed INTEGER NOT NULL DEFAULT 0 CONSTRAINT access_event_sync_runs_pages_completed_check
        CHECK (pages_completed >= 0),
    records_imported INTEGER NOT NULL DEFAULT 0 CONSTRAINT access_event_sync_runs_records_imported_check
        CHECK (records_imported >= 0),
    records_duplicate INTEGER NOT NULL DEFAULT 0 CONSTRAINT access_event_sync_runs_records_duplicate_check
        CHECK (records_duplicate >= 0),
    failure TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT access_event_sync_runs_lifecycle_check CHECK (
        (status = 'awaiting_time'
         AND search_started_at IS NULL AND search_ended_at IS NULL AND total_matches IS NULL
         AND pages_completed = 0 AND records_imported = 0 AND records_duplicate = 0
         AND failure IS NULL AND completed_at IS NULL)
        OR
        (status = 'running'
         AND search_started_at IS NOT NULL AND search_ended_at IS NOT NULL
         AND failure IS NULL AND completed_at IS NULL)
        OR
        (status = 'completed'
         AND search_started_at IS NOT NULL AND search_ended_at IS NOT NULL AND total_matches IS NOT NULL
         AND pages_completed >= 1 AND records_imported + records_duplicate = total_matches
         AND failure IS NULL AND completed_at IS NOT NULL)
        OR
        (status = 'failed' AND failure IS NOT NULL AND completed_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX access_event_sync_runs_one_active_terminal_idx
    ON access_event_sync_runs (terminal_serial_number)
    WHERE status IN ('awaiting_time', 'running');
CREATE INDEX access_event_sync_runs_terminal_created_idx
    ON access_event_sync_runs (terminal_serial_number, created_at DESC, uuid DESC);

CREATE TABLE access_event_sync_commands (
    command_uuid TEXT PRIMARY KEY REFERENCES isapi_commands(uuid) ON DELETE RESTRICT,
    sync_run_uuid TEXT NOT NULL REFERENCES access_event_sync_runs(uuid) ON DELETE RESTRICT,
    phase TEXT NOT NULL CONSTRAINT access_event_sync_commands_phase_check
        CHECK (phase IN ('time', 'page')),
    search_result_position INTEGER CONSTRAINT access_event_sync_commands_position_check
        CHECK (search_result_position BETWEEN 0 AND 150000),
    CONSTRAINT access_event_sync_commands_shape_check CHECK (
        (phase = 'time' AND search_result_position IS NULL)
        OR (phase = 'page' AND search_result_position IS NOT NULL)
    ),
    UNIQUE (sync_run_uuid, phase, search_result_position)
);

CREATE TABLE retained_access_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    terminal_serial_number TEXT NOT NULL REFERENCES terminals(serial_number),
    source_sync_run_uuid TEXT NOT NULL REFERENCES access_event_sync_runs(uuid) ON DELETE RESTRICT,
    source_command_uuid TEXT NOT NULL REFERENCES isapi_commands(uuid) ON DELETE RESTRICT,
    source_record BYTEA NOT NULL CONSTRAINT retained_access_events_source_record_check
        CHECK (octet_length(source_record) > 0),
    source_sha256 BYTEA NOT NULL CONSTRAINT retained_access_events_source_sha256_check
        CHECK (octet_length(source_sha256) = 32),
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    major_event_type SMALLINT NOT NULL CONSTRAINT retained_access_events_major_event_type_check
        CHECK (major_event_type BETWEEN 1 AND 5),
    sub_event_type INTEGER NOT NULL CONSTRAINT retained_access_events_sub_event_type_check
        CHECK (sub_event_type >= 0),
    occurred_at TIMESTAMPTZ NOT NULL,
    employee_number TEXT,
    employee_name TEXT,
    card_number TEXT,
    card_reader_number INTEGER,
    door_number INTEGER,
    event_serial_number BIGINT,
    user_type TEXT,
    current_verify_mode TEXT,
    mask TEXT,
    face_rect_height TEXT,
    face_rect_width TEXT,
    face_rect_x TEXT,
    face_rect_y TEXT,
    UNIQUE (terminal_serial_number, source_sha256)
);

CREATE INDEX retained_access_events_imported_at_idx
    ON retained_access_events (imported_at DESC, id DESC);
CREATE INDEX retained_access_events_terminal_imported_at_idx
    ON retained_access_events (terminal_serial_number, imported_at DESC, id DESC);
CREATE INDEX retained_access_events_category_subtype_idx
    ON retained_access_events (major_event_type, sub_event_type, id DESC);
