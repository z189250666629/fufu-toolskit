package activityapp

import (
	"path/filepath"
	"testing"
)

func setupCreditTestDB(t *testing.T) {
	t.Helper()
	oldDB := db
	oldTokenSvc := tokenSvc
	oldTokenConfigErr := tokenConfigErr
	testDB, err := initDB(filepath.Join(t.TempDir(), "slot.db"))
	if err != nil {
		t.Fatal(err)
	}
	db = testDB
	tokenSvc = nil
	tokenConfigErr = nil
	t.Cleanup(func() {
		_ = testDB.Close()
		db = oldDB
		tokenSvc = oldTokenSvc
		tokenConfigErr = oldTokenConfigErr
	})
}
