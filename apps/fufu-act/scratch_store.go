package activityapp

import (
	"database/sql"
	"encoding/json"
	"errors"
)

func getScratch(key string) (ScratchGame, bool) {
	g, ok, _ := lookupScratch(key)
	return g, ok
}

func lookupScratch(key string) (ScratchGame, bool, error) {
	var g ScratchGame
	err := db.QueryRow(`SELECT id,card_key,mine_pos,revealed,prize_dollars,status FROM scratch_games WHERE card_key=?`, key).Scan(&g.ID, &g.CardKey, &g.MinePos, &g.Revealed, &g.PrizeDollars, &g.Status)
	if err == nil {
		return g, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ScratchGame{}, false, nil
	}
	return ScratchGame{}, false, err
}

func insertScratchGame(key string, mines []int) error {
	mb, err := json.Marshal(mines)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO scratch_games (card_key,mine_pos) VALUES (?,?)`, key, string(mb))
	return err
}

func replaceScratchGame(key string, mines []int) error {
	mb, err := json.Marshal(mines)
	if err != nil {
		return err
	}
	return withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM scratch_games WHERE card_key=?`, key); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO scratch_games (card_key,mine_pos) VALUES (?,?)`, key, string(mb))
		return err
	})
}

func updateScratchLost(key string, revealed []int) error {
	rb, err := scratchCellsJSON(revealed)
	if err != nil {
		return err
	}
	return finishScratchRound(key, rb, 0, "lost")
}

func updateScratchProgress(key string, revealed []int, prize int, status string) error {
	rb, err := scratchCellsJSON(revealed)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE scratch_games SET revealed=?, prize_dollars=?, status=? WHERE card_key=?`, rb, prize, status, key)
	return err
}

func updateScratchWonWithCredit(key string, revealed []int, prize int, status string) error {
	rb, err := scratchCellsJSON(revealed)
	if err != nil {
		return err
	}
	return finishScratchRound(key, rb, prize, status)
}

func updateScratchCashout(key string, prize int) error {
	return finishScratchRound(key, "", prize, "cashout")
}

func updateScratchCashoutWithCredit(key string, prize int) error {
	return finishScratchRound(key, "", prize, "cashout")
}

func finishScratchRound(key, revealed string, prize int, status string) error {
	return withTx(func(tx *sql.Tx) error {
		var err error
		if revealed == "" {
			_, err = tx.Exec(`UPDATE scratch_games SET prize_dollars=?, status=? WHERE card_key=?`, prize, status, key)
		} else {
			_, err = tx.Exec(`UPDATE scratch_games SET revealed=?, prize_dollars=?, status=? WHERE card_key=?`, revealed, prize, status, key)
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE cards SET used_spins=used_spins+1,total_won=total_won+?,last_spin_at=datetime('now') WHERE card_key=?`, prize, key); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,?,0)`, key, prize); err != nil {
			return err
		}
		var totalSpins, usedSpins, totalWon int
		if err := tx.QueryRow(`SELECT total_spins,used_spins,total_won FROM cards WHERE card_key=?`, key).Scan(&totalSpins, &usedSpins, &totalWon); err != nil {
			return err
		}
		if (totalSpins <= 0 || usedSpins >= totalSpins) && totalWon > 0 {
			return enqueueCreditWith(tx, key, totalWon)
		}
		return nil
	})
}

func resetScratchTestGame(key string) error {
	return withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE credit_queue SET status='archived', error='superseded by test scratch reset' WHERE card_key=? AND status='done'`, key); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM scratch_games WHERE card_key=?`, key); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE cards SET used_spins=0,total_won=0,last_spin_at=NULL WHERE card_key=?`, key); err != nil {
			return err
		}
		return nil
	})
}

func scratchCellsJSON(cells []int) (string, error) {
	data, err := json.Marshal(cells)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
