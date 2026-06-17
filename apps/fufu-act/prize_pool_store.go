package activityapp

import (
	"database/sql"

	"fufu/poolfundcore"
)

const (
	prizePoolLedgerDeposit = "deposit"
	prizePoolLedgerPayout  = "payout"
)

func currentPrizePoolBalance() (float64, error) {
	if db == nil {
		return 0, nil
	}
	var balance float64
	if err := db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM prize_pool_ledger`).Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func recordPrizePoolDepositWith(tx *sql.Tx, plan loginCardPlan) error {
	if plan.PoolContribution.Contribution <= 0 {
		return nil
	}
	_, err := tx.Exec(
		`INSERT OR IGNORE INTO prize_pool_ledger (card_key,kind,dollars,revenue,cost,net_profit,amount) VALUES (?,?,?,?,?,?,?)`,
		plan.CardKey,
		prizePoolLedgerDeposit,
		plan.Dollars,
		plan.PoolContribution.Revenue,
		plan.PoolContribution.Cost,
		plan.PoolContribution.NetProfit,
		plan.PoolContribution.Contribution,
	)
	return err
}

func recordPrizePoolPayoutWith(tx *sql.Tx, key string, sr spinResult) error {
	if !SnapshotRuntimeConfig().DynamicPrizePool.Enabled || !isDynamicPoolPayoutPrize(sr) {
		return nil
	}
	_, err := tx.Exec(
		`INSERT INTO prize_pool_ledger (card_key,kind,amount,prize_rank,prize_label) VALUES (?,?,?,?,?)`,
		key,
		prizePoolLedgerPayout,
		-float64(sr.Dollars),
		sr.Rank,
		sr.Label,
	)
	return err
}

func isDynamicPoolPayoutPrize(sr spinResult) bool {
	return poolfundcore.IsPayoutPrize(poolfundcore.PayoutPrize{
		Type:       sr.Type,
		Dollars:    sr.Dollars,
		Rank:       sr.Rank,
		Advertised: sr.Advertised,
	})
}
