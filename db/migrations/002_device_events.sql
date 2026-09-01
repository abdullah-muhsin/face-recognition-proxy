-- The original receiver persisted only inferred attendance fields. Every
-- previous row is known to have originated from a PushSDK event, so retain the
-- event identity under the accurate name. Its source payload was not captured
-- by the retired flow and remains NULL rather than being reconstructed.
ALTER TABLE attendance_records RENAME TO device_events;
ALTER TABLE device_events RENAME CONSTRAINT attendance_records_pkey TO device_events_pkey;
ALTER TABLE device_events RENAME CONSTRAINT attendance_records_terminal_serial_number_vendor_event_id_key
    TO device_events_terminal_serial_number_vendor_event_id_key;
ALTER TABLE device_events RENAME CONSTRAINT attendance_records_terminal_serial_number_fkey
    TO device_events_terminal_serial_number_fkey;
ALTER SEQUENCE attendance_records_id_seq RENAME TO device_events_id_seq;
ALTER TABLE device_events RENAME COLUMN source_format TO data_format;
ALTER TABLE device_events DROP CONSTRAINT attendance_records_source_format_check;
ALTER TABLE device_events DROP COLUMN occurred_at;
ALTER TABLE device_events DROP COLUMN employee_number;
ALTER TABLE device_events DROP COLUMN employee_name;
ALTER TABLE device_events DROP COLUMN verification_method;
ALTER TABLE device_events DROP COLUMN attendance_status;
ALTER TABLE device_events DROP COLUMN status_value;
ALTER TABLE device_events ADD COLUMN payload_base64 TEXT;
ALTER TABLE device_events ADD CONSTRAINT device_events_data_format_check
    CHECK (data_format IN ('jsonData', 'xmlData', 'boundaryData', 'noData'));

CREATE INDEX device_events_received_at_idx ON device_events (received_at DESC);
CREATE INDEX device_events_terminal_received_at_idx ON device_events (terminal_serial_number, received_at DESC);
