package activityapp

import (
	"database/sql"
	"errors"
)

func insertLoginCard(plan loginCardPlan) error {
	return withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO cards (card_key,card_name,dollars,total_spins,source,purchase_time,subscription_id,user_id,username) VALUES (?,?,?,?,?,?,?,?,?)`,
			plan.CardKey,
			plan.CardName,
			plan.Dollars,
			plan.TotalSpins,
			plan.Source,
			nullString(plan.PurchaseTime),
			nullInt64(plan.SubscriptionID),
			nullInt64(plan.UserID),
			nullString(plan.Username),
		); err != nil {
			return err
		}
		return recordPrizePoolDepositWith(tx, plan)
	})
}

func loadSpinHistory(cardKey string) ([]map[string]any, error) {
	hist := []map[string]any{}
	rows, err := db.Query(`SELECT prize_dollars, created_at FROM spin_log WHERE card_key=? AND is_retry=0 AND prize_dollars>0 ORDER BY id DESC`, cardKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p int
		var at string
		if err := rows.Scan(&p, &at); err != nil {
			return nil, err
		}
		hist = append(hist, map[string]any{"prize_dollars": p, "created_at": at})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hist, nil
}

func getCard(key string) (Card, bool) {
	c, ok, _ := lookupCard(key)
	return c, ok
}

func lookupCard(key string) (Card, bool, error) {
	return lookupCardWhere(`card_key=?`, key)
}

func lookupCardBySubscriptionID(subscriptionID int64) (Card, bool, error) {
	if subscriptionID <= 0 {
		return Card{}, false, nil
	}
	return lookupCardWhere(`subscription_id=?`, subscriptionID)
}

func lookupCardWhere(where string, arg any) (Card, bool, error) {
	var c Card
	err := db.QueryRow(
		`SELECT card_key,card_name,dollars,total_spins,used_spins,won_jackpot,total_won,source,subscription_id,user_id,username,purchase_time,rigged FROM cards WHERE `+where,
		arg,
	).Scan(
		&c.CardKey,
		&c.CardName,
		&c.Dollars,
		&c.TotalSpins,
		&c.UsedSpins,
		&c.WonJackpot,
		&c.TotalWon,
		&c.Source,
		&c.SubscriptionID,
		&c.UserID,
		&c.Username,
		&c.PurchaseTime,
		&c.Rigged,
	)
	if err == nil {
		return c, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, false, nil
	}
	return Card{}, false, err
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt64(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}
