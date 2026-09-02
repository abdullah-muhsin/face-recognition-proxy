# Forward migrations

`db/schema.sql` is the canonical schema for a new empty database. It contains
the final `CREATE TABLE`, constraint, and index definitions; a clean install
does not replay the legacy `ALTER TABLE` history.

Files placed directly in this directory are immutable forward migrations for a
schema change made after this baseline. Keep each change self-contained and
update `db/schema.sql` in the same change so the next clean install begins at
the same schema version. A clean bootstrap records each such forward migration
as represented by the canonical schema; an existing database applies it once.

`legacy/` is the immutable migration chain used only to finish an existing
database that began before the canonical baseline. It is retained for checksum
verification and upgrade safety; do not edit, delete, or add files there.
