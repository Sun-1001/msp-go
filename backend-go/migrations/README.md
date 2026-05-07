# Go Migration Policy

P2 freezes the existing Python Alembic history as a generated Go one-step schema migration. The Go migration runner owns the full clean-database schema from `0001_initial_schema.up.sql` onward.

Current rules:

- The Alembic history has been retired from this tree; use git history or archived release artifacts if the original Python migration chain must be inspected.
- Do not regenerate `0001_initial_schema.up.sql` from Python history after cutover; add reviewed Go forward migrations instead.
- Add new Go migrations as `NNNN_description.up.sql` files in this directory.
- Run `go run ./cmd/migrate` from `backend-go/` to apply pending Go migrations.
- Rollbacks are operational: restore from backup or apply a reviewed compensating forward migration.

The first migration creates the latest Python schema in one step and includes migration seed data such as default system settings and initial knowledge graph nodes.
