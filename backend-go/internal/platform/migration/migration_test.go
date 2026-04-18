package migration

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestLoadFSSortsAndParsesMigrations(t *testing.T) {
	fsys := fstest.MapFS{
		"sql/0002_add_index.up.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
		"sql/0001_baseline.up.sql":  &fstest.MapFile{Data: []byte("SELECT 1;")},
		"sql/README.md":             &fstest.MapFile{Data: []byte("ignored")},
	}

	migrations, err := LoadFS(fsys, "sql")
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}

	gotVersions := []int64{migrations[0].Version, migrations[1].Version}
	if !reflect.DeepEqual(gotVersions, []int64{1, 2}) {
		t.Fatalf("versions = %#v, want [1 2]", gotVersions)
	}
	if migrations[0].Name != "baseline" {
		t.Fatalf("first migration name = %q", migrations[0].Name)
	}
}

func TestLoadFSRejectsDuplicateVersions(t *testing.T) {
	fsys := fstest.MapFS{
		"sql/0001_a.up.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
		"sql/0001_b.up.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
	}

	if _, err := LoadFS(fsys, "sql"); err == nil {
		t.Fatal("LoadFS() error = nil, want duplicate version error")
	}
}

func TestLoadFSReturnsReadDirError(t *testing.T) {
	if _, err := LoadFS(fstest.MapFS{}, "missing"); err == nil {
		t.Fatal("LoadFS() error = nil, want read dir error")
	} else {
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("LoadFS() error = %v, want fs.PathError wrapper", err)
		}
	}
}
