package activityapp

import (
	"database/sql"
	"errors"
)

func lookupDragonBoat(key string) (DragonBoatGame, bool, error) {
	var g DragonBoatGame
	err := db.QueryRow(`SELECT id,card_key,fishing_used,zongzi_caught,zongzi_peeled,status,removed_objects FROM dragonboat_games WHERE card_key=?`, key).Scan(&g.ID, &g.CardKey, &g.FishingUsed, &g.ZongziCaught, &g.ZongziPeeled, &g.Status, &g.RemovedObjects)
	if err == nil {
		return g, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return DragonBoatGame{}, false, nil
	}
	return DragonBoatGame{}, false, err
}

func insertDragonBoatGame(key string) error {
	_, err := db.Exec(`INSERT INTO dragonboat_games (card_key) VALUES (?)`, key)
	return err
}

func updateDragonBoatFishing(key string, fishingUsed, zongziCaught int, status, removedObjects string) error {
	return withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE dragonboat_games SET fishing_used=?, zongzi_caught=?, status=?, removed_objects=?, updated_at=datetime('now') WHERE card_key=?`, fishingUsed, zongziCaught, status, removedObjects, key); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE cards SET used_spins=?, last_spin_at=datetime('now') WHERE card_key=?`, fishingUsed, key)
		return err
	})
}

func finishDragonBoatPeel(key string, card Card, prize spinResult, fishingUsed, zongziCaught, zongziPeeled int, status string) error {
	return withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE dragonboat_games SET fishing_used=?, zongzi_caught=?, zongzi_peeled=?, status=?, updated_at=datetime('now') WHERE card_key=?`, fishingUsed, zongziCaught, zongziPeeled, status, key); err != nil {
			return err
		}
		jack := 0
		if prize.Rank == "jackpot" || prize.Dollars == 1000 {
			jack = 1
		}
		if _, err := tx.Exec(`UPDATE cards SET used_spins=?, won_jackpot=won_jackpot+?, total_won=total_won+?, last_spin_at=datetime('now') WHERE card_key=?`, fishingUsed, jack, prize.Dollars, key); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,?,0)`, key, prize.Dollars); err != nil {
			return err
		}
		if err := recordPrizePoolPayoutWith(tx, key, prize); err != nil {
			return err
		}
		totalWon := card.TotalWon + prize.Dollars
		if fishingUsed >= card.TotalSpins && zongziPeeled >= zongziCaught && totalWon > 0 {
			return enqueueCreditWith(tx, key, totalWon)
		}
		return nil
	})
}
