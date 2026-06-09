package main

import (
	"fmt"
	"fufu/auth"
	"net/http"
	"os"
	"strconv"
)

func handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(r.URL.Query().Get("token"), os.Getenv("ADMIN_TOKEN"), "Chukayu98") {
		writeJSON(w, 401, map[string]string{"error": "未授权"})
		return
	}
	writeJSON(w, 200, map[string]any{"prizeRows": queryRows(`SELECT prize_dollars, COUNT(*) as count, SUM(prize_dollars) as total FROM spin_log WHERE is_retry=0 AND prize_dollars>0 GROUP BY prize_dollars ORDER BY prize_dollars ASC`), "totalSpins": scalarInt(`SELECT COUNT(*) FROM spin_log WHERE is_retry=0`), "totalWon": scalarInt(`SELECT COALESCE(SUM(prize_dollars),0) FROM spin_log WHERE is_retry=0`), "ev": ev(), "tierRows": queryRows(`SELECT dollars, COUNT(*) as cards, SUM(total_spins) as total_spins, SUM(used_spins) as used_spins, SUM(total_won) as total_won FROM cards GROUP BY dollars ORDER BY dollars ASC`), "queueRows": queryRows(`SELECT status, COUNT(*) as count, SUM(prize_dollars) as total FROM credit_queue GROUP BY status`), "scratchRows": queryRows(`SELECT status, COUNT(*) as count, SUM(prize_dollars) as total FROM scratch_games GROUP BY status`)})
}

func handlePrizes(w http.ResponseWriter, r *http.Request) {
	prizes := []map[string]int{}
	for _, p := range prizePool {
		if p.Type == "win" {
			prizes = append(prizes, map[string]int{"dollars": p.Dollars})
		}
	}
	spinMapOut := map[string]int{}
	for dollars, spins := range spinMap {
		spinMapOut[strconv.FormatFloat(dollars, 'f', -1, 64)] = spins
	}
	writeJSON(w, 200, map[string]any{"prizes": prizes, "spinMap": spinMapOut})
}

func queryRows(q string) []map[string]any {
	rows, err := db.Query(q)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range vals {
			ptr[i] = &vals[i]
		}
		_ = rows.Scan(ptr...)
		m := map[string]any{}
		for i, c := range cols {
			switch v := vals[i].(type) {
			case []byte:
				m[c] = string(v)
			default:
				m[c] = v
			}
		}
		out = append(out, m)
	}
	return out
}

func scalarInt(q string) int { var n int; _ = db.QueryRow(q).Scan(&n); return n }

func ev() string {
	sp := scalarInt(`SELECT COUNT(*) FROM spin_log WHERE is_retry=0`)
	won := scalarInt(`SELECT COALESCE(SUM(prize_dollars),0) FROM spin_log WHERE is_retry=0`)
	if sp == 0 {
		return "0"
	}
	return fmt.Sprintf("%.4f", float64(won)/float64(sp))
}
