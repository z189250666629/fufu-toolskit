package activityapp

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
	stmts := []string{`PRAGMA journal_mode = WAL`, `CREATE TABLE IF NOT EXISTS cards (card_key TEXT PRIMARY KEY, card_name TEXT NOT NULL, dollars REAL NOT NULL, total_spins INTEGER NOT NULL, used_spins INTEGER NOT NULL DEFAULT 0, won_jackpot INTEGER NOT NULL DEFAULT 0, total_won INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')), last_spin_at TEXT, source TEXT NOT NULL DEFAULT 'act', purchase_time TEXT, rigged TEXT, subscription_id INTEGER, user_id INTEGER, username TEXT)`, `CREATE TABLE IF NOT EXISTS spin_log (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, prize_dollars INTEGER NOT NULL, is_retry INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS scratch_games (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL UNIQUE, mine_pos TEXT NOT NULL, revealed TEXT NOT NULL DEFAULT '[]', prize_dollars INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'playing', created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS dragonboat_games (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL UNIQUE, fishing_used INTEGER NOT NULL DEFAULT 0, zongzi_caught INTEGER NOT NULL DEFAULT 0, zongzi_peeled INTEGER NOT NULL DEFAULT 0, removed_objects TEXT NOT NULL DEFAULT '[]', status TEXT NOT NULL DEFAULT 'fishing', created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS credit_queue (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, prize_dollars INTEGER NOT NULL, status TEXT NOT NULL DEFAULT 'pending', retries INTEGER NOT NULL DEFAULT 0, error TEXT, created_at TEXT NOT NULL DEFAULT (datetime('now')), processed_at TEXT)`, `CREATE TABLE IF NOT EXISTS prize_pool_ledger (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, kind TEXT NOT NULL, dollars REAL NOT NULL DEFAULT 0, revenue REAL NOT NULL DEFAULT 0, cost REAL NOT NULL DEFAULT 0, net_profit REAL NOT NULL DEFAULT 0, amount REAL NOT NULL, prize_rank TEXT, prize_label TEXT, created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS scratch_prize_pool_ledger (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, kind TEXT NOT NULL, dollars REAL NOT NULL DEFAULT 0, revenue REAL NOT NULL DEFAULT 0, cost REAL NOT NULL DEFAULT 0, net_profit REAL NOT NULL DEFAULT 0, amount REAL NOT NULL, prize_rank TEXT, prize_label TEXT, created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS reward_issuance (card_key TEXT PRIMARY KEY, user_id INTEGER NOT NULL, source_subscription_id INTEGER NOT NULL DEFAULT 0, reward_plan_id INTEGER NOT NULL DEFAULT 0, reward_subscription_id INTEGER NOT NULL DEFAULT 0, reward_quota INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS reward_plan_pool (slot INTEGER PRIMARY KEY, plan_id INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL DEFAULT 'idle', lease_card_key TEXT, lease_user_id INTEGER NOT NULL DEFAULT 0, source_subscription_id INTEGER NOT NULL DEFAULT 0, reward_quota INTEGER NOT NULL DEFAULT 0, duration_unit TEXT NOT NULL DEFAULT '', duration_value INTEGER NOT NULL DEFAULT 0, custom_seconds INTEGER NOT NULL DEFAULT 0, baseline_subscription_ids TEXT NOT NULL DEFAULT '[]', next_bind_at INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL DEFAULT 0)`, `CREATE UNIQUE INDEX IF NOT EXISTS prize_pool_deposit_card_key_idx ON prize_pool_ledger(card_key) WHERE kind='deposit'`, `CREATE UNIQUE INDEX IF NOT EXISTS scratch_prize_pool_deposit_card_key_idx ON scratch_prize_pool_ledger(card_key) WHERE kind='deposit'`, `DELETE FROM credit_queue WHERE status IN ('pending','done') AND id NOT IN (SELECT min_id FROM (SELECT MIN(id) AS min_id FROM credit_queue WHERE status IN ('pending','done') GROUP BY card_key))`, `CREATE UNIQUE INDEX IF NOT EXISTS credit_queue_active_card_key_idx ON credit_queue(card_key) WHERE status IN ('pending','done')`}
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
		{table: "cards", col: "subscription_id", typ: "INTEGER"},
		{table: "cards", col: "user_id", typ: "INTEGER"},
		{table: "cards", col: "username", typ: "TEXT"},
		{table: "dragonboat_games", col: "removed_objects", typ: "TEXT NOT NULL DEFAULT '[]'"},
		{table: "reward_issuance", col: "user_id", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_issuance", col: "source_subscription_id", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_issuance", col: "reward_plan_id", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_issuance", col: "reward_subscription_id", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_issuance", col: "reward_quota", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_issuance", col: "created_at", typ: "TEXT"},
		{table: "reward_issuance", col: "updated_at", typ: "TEXT"},
		{table: "reward_plan_pool", col: "plan_id", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_plan_pool", col: "state", typ: "TEXT NOT NULL DEFAULT 'idle'"},
		{table: "reward_plan_pool", col: "lease_card_key", typ: "TEXT"},
		{table: "reward_plan_pool", col: "lease_user_id", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_plan_pool", col: "source_subscription_id", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_plan_pool", col: "reward_quota", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_plan_pool", col: "duration_unit", typ: "TEXT NOT NULL DEFAULT ''"},
		{table: "reward_plan_pool", col: "duration_value", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_plan_pool", col: "custom_seconds", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_plan_pool", col: "baseline_subscription_ids", typ: "TEXT NOT NULL DEFAULT '[]'"},
		{table: "reward_plan_pool", col: "next_bind_at", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_plan_pool", col: "created_at", typ: "INTEGER NOT NULL DEFAULT 0"},
		{table: "reward_plan_pool", col: "updated_at", typ: "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, migration := range migrations {
		if err := migrateCol(d, migration.table, migration.col, migration.typ); err != nil {
			return nil, err
		}
	}
	for _, stmt := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS cards_subscription_id_idx ON cards(subscription_id) WHERE subscription_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS reward_plan_pool_lease_card_key_idx ON reward_plan_pool(lease_card_key) WHERE lease_card_key IS NOT NULL AND lease_card_key<>''`,
	} {
		if _, err := d.Exec(stmt); err != nil {
			return nil, err
		}
	}
	if err := seedRewardPlanPool(d); err != nil {
		return nil, err
	}
	if has, err := hasColumn(d, "reward_issuance", "prize_dollars"); err != nil {
		return nil, err
	} else if has {
		if _, err := d.Exec(`UPDATE reward_issuance SET reward_quota = prize_dollars WHERE reward_quota = 0`); err != nil {
			return nil, err
		}
	}
	if _, err := d.Exec(`UPDATE reward_issuance
		SET created_at = CASE WHEN created_at IS NULL OR created_at = '' THEN datetime('now') ELSE created_at END,
		    updated_at = CASE WHEN updated_at IS NULL OR updated_at = '' THEN datetime('now') ELSE updated_at END
		WHERE created_at IS NULL OR created_at = '' OR updated_at IS NULL OR updated_at = ''`); err != nil {
		return nil, err
	}
	ready = true
	return d, nil
}

func migrateCol(d *sql.DB, table, col, typ string) error {
	has, err := hasColumn(d, table, col)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = d.Exec("ALTER TABLE " + table + " ADD COLUMN " + col + " " + typ)
	return err
}

func hasColumn(d *sql.DB, table, col string) (bool, error) {
	rows, err := d.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt, pk any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
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
