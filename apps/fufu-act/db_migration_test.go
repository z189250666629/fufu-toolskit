package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateColReturnsErrorWhenDBClosed(t *testing.T) {
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if x := recover(); x != nil {
			t.Fatalf("migrateCol should return an error, not panic: %v", x)
		}
	}()
	if err := migrateCol(d, "cards", "source", "TEXT"); err == nil {
		t.Fatal("migrateCol should report closed database errors")
	}
}

func TestInitDBReturnsDirectoryCreationError(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(parentFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	d, err := initDB(filepath.Join(parentFile, "slot.db"))

	if err == nil {
		if d != nil {
			_ = d.Close()
		}
		t.Fatal("initDB should report directory creation errors")
	}
	if !strings.Contains(err.Error(), "create database directory") {
		t.Fatalf("initDB error = %v, want create database directory context", err)
	}
}

func TestInitDBClosesDatabaseOnSchemaFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slot.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`CREATE TABLE credit_queue (id INTEGER PRIMARY KEY, card_key TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := initDB(path)

	if err == nil {
		if got != nil {
			_ = got.Close()
		}
		t.Fatal("initDB should fail when existing schema is incompatible")
	}
	if got != nil {
		t.Fatalf("initDB should return nil DB on schema failure, got %#v", got)
	}
	renamed := path + ".renamed"
	if err := os.Rename(path, renamed); err != nil {
		t.Fatalf("failed database should be closed and renameable, rename err=%v", err)
	}
}
