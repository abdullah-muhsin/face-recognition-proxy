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
