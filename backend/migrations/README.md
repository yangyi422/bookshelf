# Database migrations

Migrations are applied transactionally by `internal/database.migrate` and recorded in `schema_migrations`. Version 1 creates the initial schema; version 2 adds the recoverable trash path; version 3 stores PDF page counts. Future schema changes must add a new monotonically increasing migration.
