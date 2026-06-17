package activityapp

import (
	"database/sql"
	"errors"
)

type creditEnqueueStore interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

func enqueueCredit(key string, prize int) error {
	return enqueueCreditWith(db, key, prize)
}

func enqueueCreditWith(store creditEnqueueStore, key string, prize int) error {
	if prize <= 0 {
		return nil
	}
	var id int
	err := store.QueryRow(`SELECT id FROM credit_queue WHERE card_key=? AND status IN ('pending','done')`, key).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if id == 0 {
		_, err = store.Exec(`INSERT OR IGNORE INTO credit_queue (card_key,prize_dollars) VALUES (?,?)`, key, prize)
	}
	return err
}
