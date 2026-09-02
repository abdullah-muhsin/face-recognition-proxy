CREATE TABLE pushsdk_sessions (
    terminal_serial_number TEXT PRIMARY KEY REFERENCES terminals(serial_number) ON DELETE CASCADE,
    configuration_fingerprint TEXT NOT NULL CHECK (configuration_fingerprint ~ '^[0-9a-f]{64}$'),
    payload_mode TEXT NOT NULL CHECK (payload_mode IN ('plaintext', 'encrypted')),
    salt TEXT NOT NULL CHECK (salt ~ '^[0-9A-Za-z]{64}$'),
    login_challenge TEXT NOT NULL CHECK (login_challenge ~ '^[0-9A-Za-z]{64}$'),
    next_challenge TEXT NOT NULL DEFAULT '' CHECK (next_challenge = '' OR next_challenge ~ '^[0-9a-f]{64}$'),
    iterations INTEGER NOT NULL CHECK (iterations BETWEEN 500 AND 5000),
    created_at TIMESTAMPTZ NOT NULL,
    next_challenge_at TIMESTAMPTZ,
    authenticated BOOLEAN NOT NULL DEFAULT FALSE,
    CHECK (
        (authenticated = FALSE AND next_challenge = '' AND next_challenge_at IS NULL)
        OR
        (authenticated = TRUE AND next_challenge ~ '^[0-9a-f]{64}$' AND next_challenge_at IS NOT NULL)
    )
);
