# Go Migration Policy

P2 freezes the existing Python Alembic history as a generated Go one-step schema migration. The Go migration runner owns the full clean-database schema from `0001_initial_schema.up.sql` onward.

Current rules:

- Keep existing Alembic files in `backend/alembic/versions/` as historical reference for the generated schema.
- Rebuild `0001_initial_schema.up.sql` from a clean temporary database upgraded to Alembic head when Python history changes before cutover.
- Add new Go migrations as `NNNN_description.up.sql` files in this directory.
- Run `go run ./cmd/migrate` from `backend-go/` to apply pending Go migrations.
- Rollbacks are operational: restore from backup or apply a reviewed compensating forward migration.

The first migration creates the latest Python schema in one step and includes migration seed data such as default system settings and initial knowledge graph nodes.
