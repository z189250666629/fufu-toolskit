package main

import (
	"path/filepath"
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
