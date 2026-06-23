package activityapp

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

func TestInitDBMigratesLegacySubscriptionAndRewardSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slot.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Exec(`CREATE TABLE cards (
		card_key TEXT PRIMARY KEY,
		card_name TEXT NOT NULL,
		dollars REAL NOT NULL,
		total_spins INTEGER NOT NULL,
		used_spins INTEGER NOT NULL DEFAULT 0,
		won_jackpot INTEGER NOT NULL DEFAULT 0,
		total_won INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		last_spin_at TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`CREATE TABLE reward_issuance (
		card_key TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		prize_dollars INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO reward_issuance (card_key,user_id,prize_dollars) VALUES ('legacy-card',7,12345)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`CREATE TABLE reward_plan_pool (slot INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := initDB(path)
	if err != nil {
		t.Fatalf("initDB legacy migration: %v", err)
	}
	defer got.Close()

	for _, check := range []struct {
		table string
		col   string
	}{
		{table: "cards", col: "subscription_id"},
		{table: "cards", col: "user_id"},
		{table: "cards", col: "username"},
		{table: "reward_issuance", col: "source_subscription_id"},
		{table: "reward_issuance", col: "reward_plan_id"},
		{table: "reward_issuance", col: "reward_subscription_id"},
		{table: "reward_issuance", col: "reward_quota"},
		{table: "reward_plan_pool", col: "lease_card_key"},
		{table: "reward_plan_pool", col: "baseline_subscription_ids"},
		{table: "reward_plan_pool", col: "updated_at"},
	} {
		has, err := hasColumn(got, check.table, check.col)
		if err != nil {
			t.Fatalf("hasColumn(%s,%s): %v", check.table, check.col, err)
		}
		if !has {
			t.Fatalf("expected migrated column %s.%s", check.table, check.col)
		}
	}

	var rewardQuota int64
	if err := got.QueryRow(`SELECT reward_quota FROM reward_issuance WHERE card_key='legacy-card'`).Scan(&rewardQuota); err != nil {
		t.Fatalf("reward_issuance reward_quota: %v", err)
	}
	if rewardQuota != 12345 {
		t.Fatalf("reward_quota=%d want 12345", rewardQuota)
	}

	var poolRows int
	if err := got.QueryRow(`SELECT COUNT(*) FROM reward_plan_pool`).Scan(&poolRows); err != nil {
		t.Fatalf("reward_plan_pool count: %v", err)
	}
	if poolRows != rewardPlanPoolSize {
		t.Fatalf("reward_plan_pool rows=%d want %d", poolRows, rewardPlanPoolSize)
	}
}
