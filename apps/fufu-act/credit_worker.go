package activityapp

import "time"

func creditWorker() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		processCredits()
		<-ticker.C
	}
}

func processCredits() {
	if tokenSvc == nil || db == nil {
		return
	}
	processCreditsWith(newSQLiteCreditProcessorStore(db), newAPICreditQuotaAdapter{service: tokenSvc}, maxCreditRetries)
}

func processCreditsWith(store creditProcessorStore, quota creditQuotaAdapter, maxRetries int) {
	rows, err := store.Pending(maxRetries, creditBatchLimit)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		item, err := rows.Scan()
		if err != nil {
			if item.ID > 0 {
				_ = store.MarkScanFailed(item.ID, sanitizeCreditScanError(err))
			}
			continue
		}
		if err := quota.AddQuota(item.CardKey, item.PrizeDollars); err != nil {
			_ = store.MarkQuotaFailed(item.ID, buildCreditFailureUpdate(item.Retries, maxRetries, err))
			continue
		}
		_ = store.MarkDone(item.ID)
	}
	if err := rows.Err(); err != nil {
		return
	}
}
