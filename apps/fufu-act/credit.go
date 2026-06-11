package activityapp

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type creditQueueStore interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

var creditQuotaTimeout = 30 * time.Second

func creditQuotaContext() (context.Context, context.CancelFunc) {
	if creditQuotaTimeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), creditQuotaTimeout)
}

func sanitizeCreditScanError(error) string {
	return "队列数据异常，请人工检查"
}

func sanitizeCreditFailureError(error) string {
	return "派奖失败，请人工处理"
}

func enqueueCredit(key string, prize int) error {
	return enqueueCreditWith(db, key, prize)
}

func enqueueCreditWith(store creditQueueStore, key string, prize int) error {
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

func creditWorker() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		processCredits()
		<-ticker.C
	}
}

func processCredits() {
	if tokenSvc == nil {
		return
	}
	rows, err := db.Query(`SELECT cq.id,cq.card_key,cq.prize_dollars,cq.retries FROM credit_queue cq INNER JOIN (SELECT card_key, MIN(id) as min_id FROM credit_queue WHERE status='pending' AND retries < ? GROUP BY card_key) earliest ON cq.id=earliest.min_id ORDER BY cq.id ASC LIMIT 10`, maxCreditRetries)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, prize, retries int
		var key string
		if err := rows.Scan(&id, &key, &prize, &retries); err != nil {
			if id > 0 {
				_, _ = db.Exec(`UPDATE credit_queue SET status='failed', error=? WHERE id=?`, sanitizeCreditScanError(err), id)
			}
			continue
		}
		ctx, cancel := creditQuotaContext()
		err := tokenSvc.AddQuota(ctx, key, int64(prize))
		cancel()
		if err != nil {
			nr := retries + 1
			status := "pending"
			if nr >= maxCreditRetries {
				status = "failed"
			}
			_, _ = db.Exec(`UPDATE credit_queue SET retries=?, status=?, error=? WHERE id=?`, nr, status, sanitizeCreditFailureError(err), id)
		} else {
			_, _ = db.Exec(`UPDATE credit_queue SET status='done', error=NULL, processed_at=datetime('now') WHERE id=?`, id)
		}
	}
	if err := rows.Err(); err != nil {
		return
	}
}
