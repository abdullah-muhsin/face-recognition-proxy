ALTER TABLE isapi_commands
    ADD COLUMN response_data_format_declared BOOLEAN;

UPDATE isapi_commands
    SET response_data_format_declared = TRUE
    WHERE status = 'completed';

ALTER TABLE isapi_commands
    DROP CONSTRAINT isapi_commands_check1;

ALTER TABLE isapi_commands
    ADD CONSTRAINT isapi_commands_response_state_check
    CHECK ((
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
              AND response_data_base64 = '')
         ))
        OR
        (status = 'expired'
         AND sent_at IS NULL
         AND completed_at IS NOT NULL
         AND response_data_format IS NULL
         AND response_data_format_declared IS NULL
         AND response_data_base64 IS NULL)
    ) IS TRUE);
