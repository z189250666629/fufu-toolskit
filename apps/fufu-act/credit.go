package main

import (
	"context"
	"time"
)

func enqueueCredit(key string, prize int) {
	var id int
	_ = db.QueryRow(`SELECT id FROM credit_queue WHERE card_key=? AND status IN ('pending','done')`, key).Scan(&id)
	if id == 0 {
		_, _ = db.Exec(`INSERT OR IGNORE INTO credit_queue (card_key,prize_dollars) VALUES (?,?)`, key, prize)
	}
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
				_, _ = db.Exec(`UPDATE credit_queue SET status='failed', error=? WHERE id=?`, err.Error(), id)
			}
			continue
		}
		if err := tokenSvc.AddQuota(context.Background(), key, int64(prize)); err != nil {
			nr := retries + 1
			status := "pending"
			if nr >= maxCreditRetries {
				status = "failed"
			}
			_, _ = db.Exec(`UPDATE credit_queue SET retries=?, status=?, error=? WHERE id=?`, nr, status, err.Error(), id)
		} else {
			_, _ = db.Exec(`UPDATE credit_queue SET status='done', processed_at=datetime('now') WHERE id=?`, id)
		}
	}
	if err := rows.Err(); err != nil {
		return
	}
}
