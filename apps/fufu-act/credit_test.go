package main

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"fufu/tokens"
)

func TestProcessCreditsReturnsOnQueueQueryError(t *testing.T) {
	oldDB := db
	oldTokenSvc := tokenSvc
	testDB, err := initDB(filepath.Join(t.TempDir(), "slot.db"))
	if err != nil {
		t.Fatal(err)
	}
	db = testDB
	tokenSvc = &tokens.Service{}
	t.Cleanup(func() {
		db = oldDB
		tokenSvc = oldTokenSvc
	})

	if err := testDB.Close(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("processCredits should ignore queue query errors without panicking: %v", r)
		}
	}()
	processCredits()
}

func TestCreditQueueRejectsDuplicateActiveCardKeys(t *testing.T) {
	setupScratchLockTestDB(t)

	if _, err := db.Exec(`INSERT INTO credit_queue (card_key, prize_dollars, status) VALUES (?,?,?)`, "credit-card", 10, "pending"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO credit_queue (card_key, prize_dollars, status) VALUES (?,?,?)`, "credit-card", 20, "pending"); err == nil {
		t.Fatal("duplicate pending credit queue item should be rejected")
	}
	if _, err := db.Exec(`INSERT INTO credit_queue (card_key, prize_dollars, status) VALUES (?,?,?)`, "failed-card", 10, "failed"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO credit_queue (card_key, prize_dollars, status) VALUES (?,?,?)`, "failed-card", 20, "pending"); err != nil {
		t.Fatalf("failed items should not block a new pending queue item: %v", err)
	}
}

func TestProcessCreditsMarksMalformedQueueRowFailedOnScanError(t *testing.T) {
	setupScratchLockTestDB(t)
	oldTokenSvc := tokenSvc
	tokenSvc = &tokens.Service{}
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	res, err := db.Exec(`INSERT INTO credit_queue (card_key, prize_dollars, status) VALUES (?,?,?)`, "bad-credit-card", "bad", "pending")
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	processCredits()

	var status string
	var msg sql.NullString
	if err := db.QueryRow(`SELECT status,error FROM credit_queue WHERE id=?`, id).Scan(&status, &msg); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("status=%q error=%#v", status, msg)
	}
	if !msg.Valid || !strings.Contains(strings.ToLower(msg.String), "scan") {
		t.Fatalf("expected scan error message, got %#v", msg)
	}
}
