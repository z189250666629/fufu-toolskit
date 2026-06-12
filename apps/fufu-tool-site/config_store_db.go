package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// toolConfigDBName is the SQLite database that holds the unified admin
// configuration (NewAPI managed sites + activity config). It is the source of
// truth for admin params; environment variables only seed it on first boot.
const toolConfigDBName = "tool-config.db"

// openToolConfigDB opens (creating if needed) the SQLite config database and
// ensures the single-row tool_config table exists.
func openToolConfigDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create config database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	ready := false
	defer func() {
		if !ready {
			_ = db.Close()
		}
	}()
	stmts := []string{
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS tool_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			data TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return nil, err
		}
	}
	ready = true
	return db, nil
}

// readToolConfigRow returns the stored config JSON. ok is false when the
// database has not been seeded yet (first boot).
func readToolConfigRow(db *sql.DB) ([]byte, bool, error) {
	var data string
	err := db.QueryRow(`SELECT data FROM tool_config WHERE id = 1`).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(data), true, nil
}

// writeToolConfigRow upserts the single config row.
func writeToolConfigRow(db *sql.DB, data []byte) error {
	_, err := db.Exec(
		`INSERT INTO tool_config (id, data, updated_at) VALUES (1, ?, datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		string(data),
	)
	return err
}
