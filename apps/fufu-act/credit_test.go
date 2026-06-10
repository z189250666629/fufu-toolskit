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
