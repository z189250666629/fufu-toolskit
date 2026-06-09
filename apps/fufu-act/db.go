package main

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

func initDB(path string) (*sql.DB, error) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	stmts := []string{`PRAGMA journal_mode = WAL`, `CREATE TABLE IF NOT EXISTS cards (card_key TEXT PRIMARY KEY, card_name TEXT NOT NULL, dollars REAL NOT NULL, total_spins INTEGER NOT NULL, used_spins INTEGER NOT NULL DEFAULT 0, won_jackpot INTEGER NOT NULL DEFAULT 0, total_won INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')), last_spin_at TEXT, source TEXT NOT NULL DEFAULT 'act', purchase_time TEXT, rigged TEXT)`, `CREATE TABLE IF NOT EXISTS spin_log (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, prize_dollars INTEGER NOT NULL, is_retry INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS scratch_games (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL UNIQUE, mine_pos TEXT NOT NULL, revealed TEXT NOT NULL DEFAULT '[]', prize_dollars INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'playing', created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS credit_queue (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, prize_dollars INTEGER NOT NULL, status TEXT NOT NULL DEFAULT 'pending', retries INTEGER NOT NULL DEFAULT 0, error TEXT, created_at TEXT NOT NULL DEFAULT (datetime('now')), processed_at TEXT)`}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			return nil, err
		}
	}
	migrateCol(d, "cards", "source", "TEXT NOT NULL DEFAULT 'act'")
	migrateCol(d, "cards", "purchase_time", "TEXT")
	migrateCol(d, "cards", "rigged", "TEXT")
	return d, nil
}

func migrateCol(d *sql.DB, table, col, typ string) {
	rows, _ := d.Query("PRAGMA table_info(" + table + ")")
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt, pk any
		_ = rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		if name == col {
			return
		}
	}
	_, _ = d.Exec("ALTER TABLE " + table + " ADD COLUMN " + col + " " + typ)
}
