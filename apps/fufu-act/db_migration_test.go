package main

import (
	"database/sql"
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
