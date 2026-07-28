package migrations

import (
	"embed"

	"mathstudy/backend/internal/platform/migration"
)

//go:embed *.up.sql
var files embed.FS

// Load returns the Go-owned forward migrations embedded in this package.
func Load() ([]migration.Migration, error) {
	return migration.LoadFS(files, ".")
}
