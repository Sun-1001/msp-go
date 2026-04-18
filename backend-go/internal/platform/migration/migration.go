package migration

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var migrationFilePattern = regexp.MustCompile(`^([0-9]+)_(.+)\.up\.sql$`)

// Migration is one forward-only database migration.
type Migration struct {
	Version int64
	Name    string
	SQL     string
}

// LoadFS loads forward SQL migrations from an fs.FS directory.
func LoadFS(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migration directory: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}
		data, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(dir, entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		migrations = append(migrations, Migration{
			Version: version,
			Name:    matches[2],
			SQL:     string(data),
		})
	}

	if err := validateMigrations(migrations); err != nil {
		return nil, err
	}
	return migrations, nil
}

func validateMigrations(migrations []Migration) error {
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	seen := make(map[int64]string, len(migrations))
	for _, migration := range migrations {
		if migration.Version <= 0 {
			return fmt.Errorf("migration version must be positive, got %d", migration.Version)
		}
		if strings.TrimSpace(migration.Name) == "" {
			return fmt.Errorf("migration %d name is empty", migration.Version)
		}
		if strings.TrimSpace(migration.SQL) == "" {
			return fmt.Errorf("migration %d SQL is empty", migration.Version)
		}
		if previous, ok := seen[migration.Version]; ok {
			return fmt.Errorf("duplicate migration version %d: %s and %s", migration.Version, previous, migration.Name)
		}
		seen[migration.Version] = migration.Name
	}
	return nil
}
