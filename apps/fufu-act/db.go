package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

func initDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	ready := false
	defer func() {
		if !ready {
			_ = d.Close()
		}
	}()
	stmts := []string{`PRAGMA journal_mode = WAL`, `CREATE TABLE IF NOT EXISTS cards (card_key TEXT PRIMARY KEY, card_name TEXT NOT NULL, dollars REAL NOT NULL, total_spins INTEGER NOT NULL, used_spins INTEGER NOT NULL DEFAULT 0, won_jackpot INTEGER NOT NULL DEFAULT 0, total_won INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')), last_spin_at TEXT, source TEXT NOT NULL DEFAULT 'act', purchase_time TEXT, rigged TEXT)`, `CREATE TABLE IF NOT EXISTS spin_log (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, prize_dollars INTEGER NOT NULL, is_retry INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS scratch_games (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL UNIQUE, mine_pos TEXT NOT NULL, revealed TEXT NOT NULL DEFAULT '[]', prize_dollars INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'playing', created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS credit_queue (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, prize_dollars INTEGER NOT NULL, status TEXT NOT NULL DEFAULT 'pending', retries INTEGER NOT NULL DEFAULT 0, error TEXT, created_at TEXT NOT NULL DEFAULT (datetime('now')), processed_at TEXT)`, `DELETE FROM credit_queue WHERE status IN ('pending','done') AND id NOT IN (SELECT min_id FROM (SELECT MIN(id) AS min_id FROM credit_queue WHERE status IN ('pending','done') GROUP BY card_key))`, `CREATE UNIQUE INDEX IF NOT EXISTS credit_queue_active_card_key_idx ON credit_queue(card_key) WHERE status IN ('pending','done')`}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			return nil, err
		}
	}
	migrations := []struct {
		table string
		col   string
		typ   string
	}{
		{table: "cards", col: "source", typ: "TEXT NOT NULL DEFAULT 'act'"},
		{table: "cards", col: "purchase_time", typ: "TEXT"},
		{table: "cards", col: "rigged", typ: "TEXT"},
	}
	for _, migration := range migrations {
		if err := migrateCol(d, migration.table, migration.col, migration.typ); err != nil {
			return nil, err
		}
	}
	ready = true
	return d, nil
}

func migrateCol(d *sql.DB, table, col, typ string) error {
	rows, err := d.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt, pk any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == col {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = d.Exec("ALTER TABLE " + table + " ADD COLUMN " + col + " " + typ)
	return err
}

func withTx(fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
