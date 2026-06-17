package activityapp

import (
	"fmt"
	"fufu/activity"
	"fufu/auth"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") {
		writeJSONError(w, 401, "未授权")
		return
	}
	stats, err := buildAdminStats()
	if err != nil {
		writeJSONError(w, 500, "服务器错误")
		return
	}
	writeJSON(w, 200, stats)
}

func adminBearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if value == "" {
		return ""
	}
	scheme, token, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func handlePrizes(w http.ResponseWriter, r *http.Request) {
	cfg := SnapshotRuntimeConfig()
	spinMapOut := map[string]int{}
	for dollars, spins := range cfg.SpinMap {
		spinMapOut[strconv.FormatFloat(dollars, 'f', -1, 64)] = spins
	}
	poolBalance, err := currentPrizePoolBalance()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	pool := cfg.PrizePool
	if cfg.DynamicPrizePool.Enabled {
		pool = activity.PrizePoolWithDynamicAwards(cfg, poolBalance)
	}
	poolCfg := cfg
	poolCfg.PrizePool = pool
	balanced := activity.BalancedPrizePoolForGame(poolCfg, activity.GameSlot)
	writeJSON(w, 200, map[string]any{
		"prizes":      buildPrizeWeightRows(balanced.Pool),
		"gameConfigs": cfg.GameConfigs,
		"spinMap":     spinMapOut,
		"poolBalance": poolBalance,
	})
}

type prizeWeightResponse struct {
	Type        string `json:"type"`
	Dollars     int    `json:"dollars"`
	Weight      int    `json:"weight"`
	TotalWeight int    `json:"totalWeight"`
	Rank        string `json:"rank,omitempty"`
	Label       string `json:"label,omitempty"`
	Advertised  bool   `json:"advertised,omitempty"`
}

func buildPrizeWeightRows(pool []activity.Prize) []prizeWeightResponse {
	total := sumPoolWeights(pool)
	prizes := []prizeWeightResponse{}
	for _, p := range pool {
		if p.Type == "win" {
			prizes = append(prizes, prizeWeightResponse{
				Type:        p.Type,
				Dollars:     p.Dollars,
				Weight:      p.Weight,
				TotalWeight: total,
				Rank:        p.Rank,
				Label:       p.Label,
				Advertised:  p.Advertised,
			})
		}
	}
	return prizes
}

func sumPoolWeights(pool []activity.Prize) int {
	total := 0
	for _, p := range pool {
		total += p.Weight
	}
	return total
}

func buildAdminStats() (map[string]any, error) {
	prizeRows, err := queryRows(`SELECT prize_dollars, COUNT(*) as count, SUM(prize_dollars) as total FROM spin_log WHERE is_retry=0 AND prize_dollars>0 GROUP BY prize_dollars ORDER BY prize_dollars ASC`)
	if err != nil {
		return nil, err
	}
	totalSpins, err := scalarInt(`SELECT COUNT(*) FROM spin_log WHERE is_retry=0`)
	if err != nil {
		return nil, err
	}
	totalWon, err := scalarInt(`SELECT COALESCE(SUM(prize_dollars),0) FROM spin_log WHERE is_retry=0`)
	if err != nil {
		return nil, err
	}
	ev, err := ev()
	if err != nil {
		return nil, err
	}
	tierRows, err := queryRows(`SELECT dollars, COUNT(*) as cards, SUM(total_spins) as total_spins, SUM(used_spins) as used_spins, SUM(total_won) as total_won FROM cards GROUP BY dollars ORDER BY dollars ASC`)
	if err != nil {
		return nil, err
	}
	queueRows, err := queryRows(`SELECT status, COUNT(*) as count, SUM(prize_dollars) as total FROM credit_queue GROUP BY status`)
	if err != nil {
		return nil, err
	}
	scratchRows, err := queryRows(`SELECT status, COUNT(*) as count, SUM(prize_dollars) as total FROM scratch_games GROUP BY status`)
	if err != nil {
		return nil, err
	}
	poolBalance, err := currentPrizePoolBalance()
	if err != nil {
		return nil, err
	}
	return map[string]any{"prizeRows": prizeRows, "totalSpins": totalSpins, "totalWon": totalWon, "ev": ev, "tierRows": tierRows, "queueRows": queueRows, "scratchRows": scratchRows, "poolBalance": poolBalance}, nil
}

func queryRows(q string) ([]map[string]any, error) {
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range vals {
			ptr[i] = &vals[i]
		}
		if err := rows.Scan(ptr...); err != nil {
			return nil, err
		}
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scalarInt(q string) (int, error) {
	var n int
	if err := db.QueryRow(q).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func ev() (string, error) {
	sp, err := scalarInt(`SELECT COUNT(*) FROM spin_log WHERE is_retry=0`)
	if err != nil {
		return "", err
	}
	won, err := scalarInt(`SELECT COALESCE(SUM(prize_dollars),0) FROM spin_log WHERE is_retry=0`)
	if err != nil {
		return "", err
	}
	if sp == 0 {
		return "0", nil
	}
	return fmt.Sprintf("%.4f", float64(won)/float64(sp)), nil
}
