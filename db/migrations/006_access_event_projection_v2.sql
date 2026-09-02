ALTER TABLE access_event_projections
    DROP CONSTRAINT access_event_projections_schema_version_check;

ALTER TABLE access_event_projections
    ADD CONSTRAINT access_event_projections_schema_version_check
    CHECK (schema_version > 0);
