package activityapp

import (
	"database/sql"

	"fufu/activity"
	"fufu/poolfundcore"
)

const (
	prizePoolLedgerDeposit = "deposit"
	prizePoolLedgerPayout  = "payout"
)

func currentPrizePoolBalance() (float64, error) {
	return currentLedgerBalance("prize_pool_ledger")
}

func currentScratchPrizePoolBalance() (float64, error) {
	return currentLedgerBalance("scratch_prize_pool_ledger")
}

func currentLedgerBalance(table string) (float64, error) {
	if db == nil {
		return 0, nil
	}
	query := ""
	switch table {
	case "prize_pool_ledger":
		query = `SELECT COALESCE(SUM(amount),0) FROM prize_pool_ledger`
	case "scratch_prize_pool_ledger":
		query = `SELECT COALESCE(SUM(amount),0) FROM scratch_prize_pool_ledger`
	default:
		return 0, nil
	}
	var balance float64
	if err := db.QueryRow(query).Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func recordPrizePoolDepositWith(tx *sql.Tx, plan loginCardPlan) error {
	if plan.PoolContribution.Contribution <= 0 {
		return nil
	}
	query := `INSERT OR IGNORE INTO prize_pool_ledger (card_key,kind,dollars,revenue,cost,net_profit,amount) VALUES (?,?,?,?,?,?,?)`
	if planPrizePoolGame(plan) == activity.GameScratch {
		query = `INSERT OR IGNORE INTO scratch_prize_pool_ledger (card_key,kind,dollars,revenue,cost,net_profit,amount) VALUES (?,?,?,?,?,?,?)`
	}
	_, err := tx.Exec(
		query,
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

func planPrizePoolGame(plan loginCardPlan) string {
	if plan.Game != "" {
		return plan.Game
	}
	return SnapshotRuntimeConfig().GameForTier(plan.Dollars)
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

func recordScratchPrizePoolPayoutWith(tx *sql.Tx, key string, prize int) error {
	if !SnapshotRuntimeConfig().DynamicPrizePool.Enabled || prize <= 0 {
		return nil
	}
	_, err := tx.Exec(
		`INSERT INTO scratch_prize_pool_ledger (card_key,kind,amount,prize_rank,prize_label) VALUES (?,?,?,?,?)`,
		key,
		prizePoolLedgerPayout,
		-float64(prize),
		"scratch",
		"刮刮乐",
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
