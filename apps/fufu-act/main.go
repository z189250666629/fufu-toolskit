package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fufu/activity"
	"fufu/auth"
	"fufu/config"
	"fufu/newapi"
	"fufu/tokens"
	"fufu/webutil"
	_ "modernc.org/sqlite"
)

const (
	defaultPort            = "18820"
	actStart               = activity.StartText
	actEnd                 = activity.EndText
	actStartTS       int64 = activity.StartTS
	actEndTS         int64 = activity.EndTS
	maxCreditRetries       = 5
)

var rootDir string
var db *sql.DB
var tokenSvc *tokens.Service
var tokenConfigErr error
var cardLocks sync.Map
var mcyCookie string

var spinMap = activity.SpinMap
var prizePool = activity.PrizePool
var scratchRewards = activity.ScratchRewards

const scratchMines = activity.ScratchMines
const scratchMaxReveals = activity.ScratchMaxReveals

type Card struct {
	CardKey      string
	CardName     string
	Dollars      float64
	TotalSpins   int
	UsedSpins    int
	WonJackpot   int
	TotalWon     int
	Source       string
	PurchaseTime sql.NullString
	Rigged       sql.NullString
}
type ScratchGame struct {
	ID           int
	CardKey      string
	MinePos      string
	Revealed     string
	PrizeDollars int
	Status       string
}

func main() {
	wd, _ := os.Getwd()
	rootDir = wd
	if err := initAll(); err != nil {
		panic(err)
	}
	defer db.Close()
	go creditWorker()
	port := strings.TrimSpace(os.Getenv("SLOT_PORT"))
	if port == "" {
		port = defaultPort
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", apiRoute)
	mux.HandleFunc("/", staticRoute)
	fmt.Printf("fufu-act Go backend listening on :%s\n", port)
	panic(http.ListenAndServe("0.0.0.0:"+port, mux))
}
func initAll() error {
	var err error
	db, err = initDB(filepath.Join(rootDir, "data", "slot.db"))
	if err != nil {
		return err
	}
	site, err := config.LoadPrimarySite(rootDir)
	if err != nil {
		tokenConfigErr = err
	} else {
		tokenSvc = tokens.NewService(newapi.NewClient(site))
	}
	mcyCookie = config.Env("MCY_COOKIE")
	return nil
}
func initDB(path string) (*sql.DB, error) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	stmts := []string{`PRAGMA journal_mode = WAL`, `CREATE TABLE IF NOT EXISTS cards (card_key TEXT PRIMARY KEY, card_name TEXT NOT NULL, dollars REAL NOT NULL, total_spins INTEGER NOT NULL, used_spins INTEGER NOT NULL DEFAULT 0, won_jackpot INTEGER NOT NULL DEFAULT 0, total_won INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')), last_spin_at TEXT, source TEXT NOT NULL DEFAULT 'act', purchase_time TEXT, rigged TEXT)`, `CREATE TABLE IF NOT EXISTS spin_log (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, prize_dollars INTEGER NOT NULL, is_retry INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS scratch_games (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL UNIQUE, mine_pos TEXT NOT NULL, revealed TEXT NOT NULL DEFAULT '[]', prize_dollars INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'playing', created_at TEXT NOT NULL DEFAULT (datetime('now')))`, `CREATE TABLE IF NOT EXISTS credit_queue (id INTEGER PRIMARY KEY AUTOINCREMENT, card_key TEXT NOT NULL, prize_dollars INTEGER NOT NULL, status TEXT NOT NULL DEFAULT 'pending', retries INTEGER NOT NULL DEFAULT 0, error TEXT, created_at TEXT NOT NULL DEFAULT (datetime('now')), processed_at TEXT)`}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			return nil, err
		}
	}
	migrateCol(d, "cards", "source", "TEXT NOT NULL DEFAULT 'act'")
	migrateCol(d, "cards", "purchase_time", "TEXT")
	migrateCol(d, "cards", "rigged", "TEXT")
	return d, nil
}
func migrateCol(d *sql.DB, table, col, typ string) {
	rows, _ := d.Query("PRAGMA table_info(" + table + ")")
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt, pk any
		_ = rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		if name == col {
			return
		}
	}
	_, _ = d.Exec("ALTER TABLE " + table + " ADD COLUMN " + col + " " + typ)
}

func apiRoute(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/login":
		post(w, r, handleLogin)
	case "/api/spin":
		post(w, r, handleSpin)
	case "/api/scratch/start":
		post(w, r, handleScratchStart)
	case "/api/scratch/reveal":
		post(w, r, handleScratchReveal)
	case "/api/scratch/cashout":
		post(w, r, handleScratchCashout)
	case "/api/scratch/reset":
		post(w, r, handleScratchReset)
	case "/api/admin/stats":
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "Only GET"})
			return
		}
		handleAdminStats(w, r)
	case "/api/prizes":
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "Only GET"})
			return
		}
		handlePrizes(w, r)
	default:
		writeJSON(w, 404, map[string]string{"error": "Not found"})
	}
}
func post(w http.ResponseWriter, r *http.Request, fn func(http.ResponseWriter, *http.Request)) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Only POST"})
		return
	}
	fn(w, r)
}
func staticRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, 405, map[string]string{"error": "Only GET"})
		return
	}
	p := r.URL.Path
	if p == "/" {
		p = "/index.html"
	}
	publicDir := filepath.Join(rootDir, "public")
	file, ok := webutil.SafePath(publicDir, p)
	if !ok {
		http.Error(w, "Forbidden", 403)
		return
	}
	if _, err := os.Stat(file); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, file)
}
func writeJSON(w http.ResponseWriter, status int, payload any) {
	webutil.WriteJSON(w, status, payload)
}
func readBody(r *http.Request, out any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(out)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CardKey string `json:"cardKey"`
	}
	if readBody(r, &body) != nil || strings.TrimSpace(body.CardKey) == "" {
		writeJSON(w, 400, map[string]string{"error": "请输入卡密"})
		return
	}
	key := strings.TrimSpace(body.CardKey)
	card, ok := getCard(key)
	if !ok {
		if tokenSvc == nil {
			writeJSON(w, 503, map[string]string{"error": "NewAPI 未配置: " + errString(tokenConfigErr)})
			return
		}
		t, err := tokenSvc.SearchTokenByKey(r.Context(), key)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if t == nil {
			writeJSON(w, 404, map[string]string{"error": "卡密不存在"})
			return
		}
		isActTest := strings.Contains(t.Name, "-act-") && strings.Contains(t.Name, "test")
		dollars := 0.0
		source := "shop"
		purchaseTime := ""
		createdInRange := false
		if isActTest {
			parts := strings.Split(t.Name, "-act-")
			dollars, _ = strconv.ParseFloat(parts[0], 64)
			source = "act"
			createdInRange = true
		} else {
			createdInRange = t.CreatedTime >= actStartTS && t.CreatedTime <= actEndTS
			shop := findShopPurchase(key)
			purchased := shop != "" && shop >= actStart && shop <= actEnd
			if !createdInRange && !purchased {
				writeJSON(w, 403, map[string]string{"error": "此卡密不在活动期间内，不参与活动"})
				return
			}
			dollars = dollarsTier(t.IntervalQuota)
			purchaseTime = shop
		}
		isScratch := int(math.Round(dollars)) == 55 && (purchaseTime != "" || createdInRange)
		if dollars == 0 || (spinMap[dollars] == 0 && !isScratch) {
			writeJSON(w, 403, map[string]string{"error": "此卡密额度不参与活动"})
			return
		}
		total := spinMap[dollars]
		_, err = db.Exec(`INSERT INTO cards (card_key,card_name,dollars,total_spins,source,purchase_time) VALUES (?,?,?,?,?,?)`, key, t.Name, dollars, total, source, nullString(purchaseTime))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		card, _ = getCard(key)
	}
	respondCard(w, card)
}
func respondCard(w http.ResponseWriter, card Card) {
	hist := []map[string]any{}
	rows, _ := db.Query(`SELECT prize_dollars, created_at FROM spin_log WHERE card_key=? AND is_retry=0 AND prize_dollars>0 ORDER BY id DESC`, card.CardKey)
	for rows.Next() {
		var p int
		var at string
		_ = rows.Scan(&p, &at)
		hist = append(hist, map[string]any{"prize_dollars": p, "created_at": at})
	}
	rows.Close()
	isScratch := int(math.Round(card.Dollars)) == 55
	var sg any
	if isScratch {
		if g, ok := getScratch(card.CardKey); ok {
			gameOver := g.Status == "won" || g.Status == "lost" || g.Status == "cashout"
			m := map[string]any{"revealed": jsonArr(g.Revealed), "prize": g.PrizeDollars, "status": g.Status}
			if gameOver {
				m["mines"] = jsonArr(g.MinePos)
			}
			sg = m
		}
	}
	writeJSON(w, 200, map[string]any{"cardKey": card.CardKey, "cardName": card.CardName, "dollars": card.Dollars, "totalSpins": card.TotalSpins, "usedSpins": card.UsedSpins, "remainingSpins": card.TotalSpins - card.UsedSpins, "totalWon": card.TotalWon, "wonJackpot": card.WonJackpot != 0, "history": hist, "isScratch": isScratch, "scratchGame": sg})
}
func getCard(key string) (Card, bool) {
	var c Card
	err := db.QueryRow(`SELECT card_key,card_name,dollars,total_spins,used_spins,won_jackpot,total_won,source,purchase_time,rigged FROM cards WHERE card_key=?`, key).Scan(&c.CardKey, &c.CardName, &c.Dollars, &c.TotalSpins, &c.UsedSpins, &c.WonJackpot, &c.TotalWon, &c.Source, &c.PurchaseTime, &c.Rigged)
	return c, err == nil
}
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func dollarsTier(q int64) float64 {
	unit := int64(newapi.DefaultQuotaUnit)
	if tokenSvc != nil && tokenSvc.QuotaUnit > 0 {
		unit = tokenSvc.QuotaUnit
	}
	return activity.DollarsTier(q, unit)
}

func handleSpin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CardKey string `json:"cardKey"`
	}
	if readBody(r, &body) != nil || body.CardKey == "" {
		writeJSON(w, 400, map[string]string{"error": "请输入卡密"})
		return
	}
	key := strings.TrimSpace(body.CardKey)
	res, err := withCardLock(key, func() (any, error) {
		card, ok := getCard(key)
		if !ok {
			return nil, httpErr{404, "请先登录"}
		}
		remaining := card.TotalSpins - card.UsedSpins
		if remaining <= 0 {
			return nil, httpErr{403, "抽奖次数已用完"}
		}
		maxWon := 0
		_ = db.QueryRow(`SELECT COALESCE(MAX(prize_dollars),0) FROM spin_log WHERE card_key=? AND is_retry=0`, key).Scan(&maxWon)
		force := 0
		if card.Rigged.Valid {
			var m map[string]int
			if json.Unmarshal([]byte(card.Rigged.String), &m) == nil {
				force = m[strconv.Itoa(card.UsedSpins+1)]
			}
		}
		sr := spin(card.Dollars, card.WonJackpot != 0, card.UsedSpins, card.TotalSpins, maxWon, force)
		if sr.Type == "retry" {
			_, _ = db.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,0,1)`, key)
			return map[string]any{"isRetry": true, "isMiss": false, "message": "再来一次！", "remainingSpins": remaining}, nil
		}
		tx, _ := db.Begin()
		if sr.Type == "miss" {
			_, _ = tx.Exec(`UPDATE cards SET used_spins=used_spins+1,last_spin_at=datetime('now') WHERE card_key=?`, key)
			_, _ = tx.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,0,0)`, key)
		} else {
			jack := 0
			if sr.Dollars == 1000 {
				jack = 1
			}
			_, _ = tx.Exec(`UPDATE cards SET used_spins=used_spins+1, won_jackpot=won_jackpot+?, total_won=total_won+?, last_spin_at=datetime('now') WHERE card_key=?`, jack, sr.Dollars, key)
			_, _ = tx.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,?,0)`, key, sr.Dollars)
		}
		_ = tx.Commit()
		updated, _ := getCard(key)
		newRem := updated.TotalSpins - updated.UsedSpins
		if newRem <= 0 && updated.TotalWon > 0 {
			enqueueCredit(key, updated.TotalWon)
		}
		if sr.Type == "miss" {
			return map[string]any{"isRetry": false, "isMiss": true, "prize": 0, "remainingSpins": newRem, "totalWon": updated.TotalWon}, nil
		}
		return map[string]any{"isRetry": false, "prize": sr.Dollars, "remainingSpins": newRem, "totalWon": updated.TotalWon, "wonJackpot": updated.WonJackpot != 0}, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}

type spinResult struct {
	Type    string
	Dollars int
}

func spin(dollars float64, hasJackpot bool, used, total, maxWon, force int) spinResult {
	got := activity.Spin(dollars, hasJackpot, used, total, maxWon, force, secureRandomInt)
	return spinResult{got.Type, got.Dollars}
}
func secureRandomInt(max int) int {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return int(binary.BigEndian.Uint32(b[:]) % uint32(max))
}

func handleScratchStart(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey string `json:"cardKey"`
	}
	if readBody(r, &b) != nil || b.CardKey == "" {
		writeJSON(w, 400, map[string]string{"error": "请输入卡密"})
		return
	}
	key := strings.TrimSpace(b.CardKey)
	card, ok := getCard(key)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "请先登录"})
		return
	}
	if int(math.Round(card.Dollars)) != 55 {
		writeJSON(w, 403, map[string]string{"error": "此卡密不参与刮刮乐活动"})
		return
	}
	if g, ok := getScratch(key); ok {
		writeJSON(w, 200, map[string]any{"cells": 9, "revealed": jsonArr(g.Revealed), "prize": g.PrizeDollars, "status": g.Status})
		return
	}
	mines := []int{}
	for len(mines) < scratchMines {
		p := secureRandomInt(9)
		exists := false
		for _, m := range mines {
			if m == p {
				exists = true
			}
		}
		if !exists {
			mines = append(mines, p)
		}
	}
	mb, _ := json.Marshal(mines)
	_, _ = db.Exec(`INSERT INTO scratch_games (card_key,mine_pos) VALUES (?,?)`, key, string(mb))
	writeJSON(w, 200, map[string]any{"cells": 9, "revealed": []int{}, "prize": 0, "status": "playing"})
}
func handleScratchReveal(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey   string `json:"cardKey"`
		CellIndex int    `json:"cellIndex"`
	}
	if readBody(r, &b) != nil || b.CardKey == "" {
		writeJSON(w, 400, map[string]string{"error": "请输入卡密"})
		return
	}
	if b.CellIndex < 0 || b.CellIndex > 8 {
		writeJSON(w, 400, map[string]string{"error": "无效的格子"})
		return
	}
	key := strings.TrimSpace(b.CardKey)
	res, err := withCardLock(key, func() (any, error) {
		g, ok := getScratch(key)
		if !ok {
			return nil, httpErr{404, "请先开始刮刮乐"}
		}
		if g.Status != "playing" {
			return nil, httpErr{403, "游戏已结束"}
		}
		revealed := jsonArr(g.Revealed)
		for _, v := range revealed {
			if v == b.CellIndex {
				return nil, httpErr{400, "此格已刮开"}
			}
		}
		mines := jsonArr(g.MinePos)
		revealed = append(revealed, b.CellIndex)
		if intContains(mines, b.CellIndex) {
			rb, _ := json.Marshal(revealed)
			_, _ = db.Exec(`UPDATE scratch_games SET revealed=?, prize_dollars=0, status='lost' WHERE card_key=?`, string(rb), key)
			return map[string]any{"hit": true, "mines": mines, "prize": 0, "status": "lost", "revealed": revealed}, nil
		}
		safe := 0
		for _, v := range revealed {
			if !intContains(mines, v) {
				safe++
			}
		}
		prize := scratchRewards[safe-1]
		status := "playing"
		if safe >= scratchMaxReveals {
			status = "won"
		}
		rb, _ := json.Marshal(revealed)
		_, _ = db.Exec(`UPDATE scratch_games SET revealed=?, prize_dollars=?, status=? WHERE card_key=?`, string(rb), prize, status, key)
		if status == "won" && prize > 0 {
			enqueueCredit(key, prize)
		}
		return map[string]any{"hit": false, "prize": prize, "status": status, "revealed": revealed}, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}
func handleScratchCashout(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey string `json:"cardKey"`
	}
	if readBody(r, &b) != nil || b.CardKey == "" {
		writeJSON(w, 400, map[string]string{"error": "请输入卡密"})
		return
	}
	key := strings.TrimSpace(b.CardKey)
	res, err := withCardLock(key, func() (any, error) {
		g, ok := getScratch(key)
		if !ok {
			return nil, httpErr{404, "请先开始刮刮乐"}
		}
		if g.Status != "playing" {
			return nil, httpErr{403, "游戏已结束"}
		}
		revealed := jsonArr(g.Revealed)
		mines := jsonArr(g.MinePos)
		safe := 0
		for _, v := range revealed {
			if !intContains(mines, v) {
				safe++
			}
		}
		if safe == 0 {
			return nil, httpErr{400, "至少刮开一个安全格才能结算"}
		}
		prize := scratchRewards[safe-1]
		_, _ = db.Exec(`UPDATE scratch_games SET prize_dollars=?, status='cashout' WHERE card_key=?`, prize, key)
		if prize > 0 {
			enqueueCredit(key, prize)
		}
		return map[string]any{"prize": prize, "status": "cashout", "revealed": revealed, "mines": mines}, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}
func handleScratchReset(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey string `json:"cardKey"`
	}
	if readBody(r, &b) != nil || b.CardKey == "" {
		writeJSON(w, 400, map[string]string{"error": "请输入卡密"})
		return
	}
	key := strings.TrimSpace(b.CardKey)
	card, ok := getCard(key)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "请先登录"})
		return
	}
	if !strings.Contains(card.CardName, "test") {
		writeJSON(w, 403, map[string]string{"error": "仅测试卡可重开"})
		return
	}
	if g, ok := getScratch(key); ok && g.Status == "playing" {
		writeJSON(w, 400, map[string]string{"error": "当前游戏尚未结束"})
		return
	}
	_, _ = db.Exec(`DELETE FROM scratch_games WHERE card_key=?`, key)
	writeJSON(w, 200, map[string]any{"ok": true})
}
func getScratch(key string) (ScratchGame, bool) {
	var g ScratchGame
	err := db.QueryRow(`SELECT id,card_key,mine_pos,revealed,prize_dollars,status FROM scratch_games WHERE card_key=?`, key).Scan(&g.ID, &g.CardKey, &g.MinePos, &g.Revealed, &g.PrizeDollars, &g.Status)
	return g, err == nil
}
func jsonArr(s string) []int {
	var a []int
	_ = json.Unmarshal([]byte(s), &a)
	if a == nil {
		return []int{}
	}
	return a
}
func intContains(a []int, v int) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}

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

func enqueueCredit(key string, prize int) {
	var id int
	_ = db.QueryRow(`SELECT id FROM credit_queue WHERE card_key=? AND status IN ('pending','done')`, key).Scan(&id)
	if id == 0 {
		_, _ = db.Exec(`INSERT INTO credit_queue (card_key,prize_dollars) VALUES (?,?)`, key, prize)
	}
}
func creditWorker() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		processCredits()
		<-ticker.C
	}
}
func processCredits() {
	if tokenSvc == nil {
		return
	}
	rows, _ := db.Query(`SELECT cq.id,cq.card_key,cq.prize_dollars,cq.retries FROM credit_queue cq INNER JOIN (SELECT card_key, MIN(id) as min_id FROM credit_queue WHERE status='pending' AND retries < ? GROUP BY card_key) earliest ON cq.id=earliest.min_id ORDER BY cq.id ASC LIMIT 10`, maxCreditRetries)
	defer rows.Close()
	for rows.Next() {
		var id, prize, retries int
		var key string
		_ = rows.Scan(&id, &key, &prize, &retries)
		if err := tokenSvc.AddQuota(context.Background(), key, int64(prize)); err != nil {
			nr := retries + 1
			status := "pending"
			if nr >= maxCreditRetries {
				status = "failed"
			}
			_, _ = db.Exec(`UPDATE credit_queue SET retries=?, status=?, error=? WHERE id=?`, nr, status, err.Error(), id)
		} else {
			_, _ = db.Exec(`UPDATE credit_queue SET status='done', processed_at=datetime('now') WHERE id=?`, id)
		}
	}
}

type httpErr struct {
	Status  int
	Message string
}

func (e httpErr) Error() string { return e.Message }
func writeHTTPError(w http.ResponseWriter, err error) {
	if e, ok := err.(httpErr); ok {
		writeJSON(w, e.Status, map[string]string{"error": e.Message})
	} else {
		writeJSON(w, 500, map[string]string{"error": "服务器错误"})
	}
}
func withCardLock(key string, fn func() (any, error)) (any, error) {
	muIface, _ := cardLocks.LoadOrStore(key, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func findShopPurchase(cardKey string) string {
	if config.Env("MCY_BASE_URL") == "" && config.Env("SHOP_BASE_URL") == "" {
		return ""
	}
	if mcyCookie == "" {
		_ = mcyLogin()
	}
	if mcyCookie == "" {
		return ""
	}
	data, err := mcyPost("/plugin/virtual-card-ship/card/get", map[string]any{"equal-card": cardKey, "page": 1, "limit": 1})
	if err != nil {
		return ""
	}
	if d, ok := data["data"].(map[string]any); ok {
		if arr, ok := d["list"].([]any); ok && len(arr) > 0 {
			if obj, ok := arr[0].(map[string]any); ok {
				return fmt.Sprint(obj["purchase_time"])
			}
		}
	}
	return ""
}
func mcyConfig() (string, string, string, string) {
	return strings.TrimRight(firstNonEmpty(config.Env("MCY_BASE_URL"), config.Env("SHOP_BASE_URL")), "/"), firstNonEmpty(config.Env("MCY_USERNAME"), config.Env("SHOP_USERNAME")), firstNonEmpty(config.Env("MCY_PASSWORD"), config.Env("SHOP_PASSWORD")), firstNonEmpty(config.Env("MCY_LOGIN_ENDPOINT"), "/admin/login")
}
func mcyLogin() error {
	base, user, pass, login := mcyConfig()
	if base == "" || user == "" || pass == "" {
		return fmt.Errorf("missing MCY config")
	}
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	resp, err := http.Post(base+login, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if sc := resp.Header.Get("Set-Cookie"); sc != "" {
		parts := []string{}
		for _, p := range strings.Split(sc, ",") {
			parts = append(parts, strings.Split(p, ";")[0])
		}
		mcyCookie = strings.Join(parts, "; ")
	}
	return nil
}
func mcyPost(endpoint string, payload any) (map[string]any, error) {
	base, _, _, _ := mcyConfig()
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, base+endpoint, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", mcyCookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&data)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, fmt.Errorf("MCY HTTP %d", resp.StatusCode)
	}
	return data, nil
}

var _ = sort.Strings

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
