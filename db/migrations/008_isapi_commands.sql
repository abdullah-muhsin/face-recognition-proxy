CREATE TABLE isapi_commands (
    uuid TEXT PRIMARY KEY CHECK (uuid ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'),
    terminal_serial_number TEXT NOT NULL REFERENCES terminals(serial_number),
    created_by_admin_user_id BIGINT REFERENCES admin_users(id) ON DELETE SET NULL,
    created_by_username TEXT NOT NULL CHECK (length(created_by_username) > 0),
    method TEXT NOT NULL CHECK (method IN ('GET', 'POST', 'PUT', 'DELETE')),
    url TEXT NOT NULL CHECK (length(url) BETWEEN 8 AND 4096 AND left(url, 7) = '/ISAPI/'),
    data_format TEXT NOT NULL CHECK (data_format IN ('jsonData', 'xmlData', 'boundaryData', 'noData')),
    request_data BYTEA NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'sent', 'completed', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    response_data_format TEXT CHECK (response_data_format IN ('jsonData', 'xmlData', 'boundaryData', 'noData')),
    response_data_base64 TEXT,
    CHECK (
        (data_format = 'noData' AND octet_length(request_data) = 0)
        OR (data_format <> 'noData' AND octet_length(request_data) > 0)
    ),
    CHECK (
        (status = 'queued' AND sent_at IS NULL AND completed_at IS NULL AND response_data_format IS NULL AND response_data_base64 IS NULL)
        OR (status = 'sent' AND sent_at IS NOT NULL AND completed_at IS NULL AND response_data_format IS NULL AND response_data_base64 IS NULL)
        OR (status = 'completed' AND sent_at IS NOT NULL AND completed_at IS NOT NULL AND response_data_format IS NOT NULL AND response_data_base64 IS NOT NULL)
        OR (status = 'expired' AND sent_at IS NULL AND completed_at IS NOT NULL AND response_data_format IS NULL AND response_data_base64 IS NULL)
    ),
    CHECK (expires_at > created_at)
);

CREATE INDEX isapi_commands_terminal_created_idx
    ON isapi_commands (terminal_serial_number, created_at DESC, uuid DESC);

CREATE INDEX isapi_commands_terminal_status_created_idx
    ON isapi_commands (terminal_serial_number, status, created_at ASC, uuid ASC);
