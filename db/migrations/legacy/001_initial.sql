CREATE TABLE terminals (
    serial_number TEXT PRIMARY KEY,
    pushsdk_serial TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    credential_fingerprint TEXT NOT NULL,
    security_version SMALLINT NOT NULL CHECK (security_version IN (3, 4)),
    command_interval_seconds INTEGER NOT NULL CHECK (command_interval_seconds BETWEEN 1 AND 300),
    error_delay_seconds INTEGER NOT NULL CHECK (error_delay_seconds BETWEEN 1 AND 300),
    connection_status TEXT NOT NULL DEFAULT 'offline' CHECK (connection_status IN ('offline', 'authenticating', 'online', 'error')),
    last_seen_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE attendance_records (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    terminal_serial_number TEXT NOT NULL REFERENCES terminals(serial_number),
    vendor_event_id TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    employee_number TEXT NOT NULL,
    employee_name TEXT,
    verification_method TEXT NOT NULL,
    attendance_status TEXT,
    status_value INTEGER,
    source_format TEXT NOT NULL CHECK (source_format IN ('jsonData', 'xmlData', 'boundaryData')),
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (terminal_serial_number, vendor_event_id)
);
CREATE INDEX attendance_records_occurred_at_idx ON attendance_records (occurred_at DESC);
CREATE INDEX attendance_records_terminal_occurred_at_idx ON attendance_records (terminal_serial_number, occurred_at DESC);

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
