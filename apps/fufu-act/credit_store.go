package activityapp

import (
	"database/sql"
)

const creditBatchLimit = 10

type creditQueueItem struct {
	ID           int
	CardKey      string
	PrizeDollars int
	Retries      int
}

type creditPendingRows interface {
	Next() bool
	Scan() (creditQueueItem, error)
	Close() error
	Err() error
}

type creditProcessorStore interface {
	Pending(maxRetries, limit int) (creditPendingRows, error)
	MarkScanFailed(id int, message string) error
	MarkQuotaFailed(id int, update creditFailureUpdate) error
	MarkDone(id int) error
}

type sqliteCreditProcessorStore struct {
	db *sql.DB
}

func newSQLiteCreditProcessorStore(db *sql.DB) sqliteCreditProcessorStore {
	return sqliteCreditProcessorStore{db: db}
}

func (s sqliteCreditProcessorStore) Pending(maxRetries, limit int) (creditPendingRows, error) {
	rows, err := s.db.Query(`SELECT cq.id,cq.card_key,cq.prize_dollars,cq.retries FROM credit_queue cq INNER JOIN (SELECT card_key, MIN(id) as min_id FROM credit_queue WHERE status='pending' AND retries < ? GROUP BY card_key) earliest ON cq.id=earliest.min_id ORDER BY cq.id ASC LIMIT ?`, maxRetries, limit)
	if err != nil {
		return nil, err
	}
	return sqliteCreditRows{rows: rows}, nil
}

func (s sqliteCreditProcessorStore) MarkScanFailed(id int, message string) error {
	_, err := s.db.Exec(`UPDATE credit_queue SET status=?, error=? WHERE id=?`, creditStatusFailed, message, id)
	return err
}

func (s sqliteCreditProcessorStore) MarkQuotaFailed(id int, update creditFailureUpdate) error {
	_, err := s.db.Exec(`UPDATE credit_queue SET retries=?, status=?, error=? WHERE id=?`, update.Retries, update.Status, update.Error, id)
	return err
}

func (s sqliteCreditProcessorStore) MarkDone(id int) error {
	_, err := s.db.Exec(`UPDATE credit_queue SET status=?, error=NULL, processed_at=datetime('now') WHERE id=?`, creditStatusDone, id)
	return err
}

type sqliteCreditRows struct {
	rows *sql.Rows
}

func (r sqliteCreditRows) Next() bool {
	return r.rows.Next()
}

func (r sqliteCreditRows) Scan() (creditQueueItem, error) {
	var item creditQueueItem
	err := r.rows.Scan(&item.ID, &item.CardKey, &item.PrizeDollars, &item.Retries)
	return item, err
}

func (r sqliteCreditRows) Close() error {
	return r.rows.Close()
}

func (r sqliteCreditRows) Err() error {
	return r.rows.Err()
}
