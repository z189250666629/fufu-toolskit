package activityapp

func maxSpinPrize(key string) (int, error) {
	maxWon := 0
	err := db.QueryRow(`SELECT COALESCE(MAX(prize_dollars),0) FROM spin_log WHERE card_key=? AND is_retry=0`, key).Scan(&maxWon)
	return maxWon, err
}

func recordSpinRetry(key string) error {
	_, err := db.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,0,1)`, key)
	return err
}

func recordSpinResult(key string, card Card, sr spinResult, remaining int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	newRem := remaining - 1
	creditPrize := card.TotalWon
	if sr.Type == "miss" {
		if _, err := tx.Exec(`UPDATE cards SET used_spins=used_spins+1,last_spin_at=datetime('now') WHERE card_key=?`, key); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,0,0)`, key); err != nil {
			return err
		}
	} else {
		jack := 0
		if sr.Rank == "jackpot" || sr.Dollars == 1000 {
			jack = 1
		}
		if _, err := tx.Exec(`UPDATE cards SET used_spins=used_spins+1, won_jackpot=won_jackpot+?, total_won=total_won+?, last_spin_at=datetime('now') WHERE card_key=?`, jack, sr.Dollars, key); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,?,0)`, key, sr.Dollars); err != nil {
			return err
		}
		if err := recordPrizePoolPayoutWith(tx, key, sr); err != nil {
			return err
		}
		creditPrize += sr.Dollars
	}
	if newRem <= 0 && creditPrize > 0 {
		if err := enqueueCreditWith(tx, key, creditPrize); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
