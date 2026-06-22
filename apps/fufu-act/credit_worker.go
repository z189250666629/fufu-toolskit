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
	tokenService, _ := snapshotTokenRuntime()
	site, subscriptionClient, _ := snapshotSubscriptionRuntime()
	if db == nil || (tokenService == nil && subscriptionClient == nil) {
		return
	}
	processCreditsWith(newSQLiteCreditProcessorStore(db), newAPICreditQuotaAdapter{
		service:               tokenService,
		subscriptionClient:    subscriptionClient,
		subscriptionQuotaUnit: quotaUnitOrDefault(site.QuotaUnit),
	}, maxCreditRetries)
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
		card, err := creditCardTarget(item.CardKey)
		if err != nil {
			_ = store.MarkQuotaFailed(item.ID, buildCreditFailureUpdate(item.Retries, maxRetries, err))
			continue
		}
		if err := quota.AddQuota(card, item.PrizeDollars); err != nil {
			failureMaxRetries := maxRetries
			if isPermanentCreditError(err) {
				failureMaxRetries = 1
			}
			_ = store.MarkQuotaFailed(item.ID, buildCreditFailureUpdate(item.Retries, failureMaxRetries, err))
			continue
		}
		_ = store.MarkDone(item.ID)
	}
	if err := rows.Err(); err != nil {
		return
	}
}

func creditCardTarget(key string) (Card, error) {
	card := Card{CardKey: key}
	if db == nil {
		return card, nil
	}
	stored, ok, err := lookupCard(key)
	if err != nil {
		return Card{}, err
	}
	if ok {
		return stored, nil
	}
	return card, nil
}
