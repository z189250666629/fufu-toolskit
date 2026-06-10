package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"fufu/activity"
	"fufu/config"
	"fufu/newapi"
	"fufu/tokens"
	"fufu/webutil"
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
var cardLocks = &cardLockRegistry{}
var mcyCookie string

var spinMap = activity.DefaultSpinMap()
var prizePool = activity.DefaultPrizePool()
var scratchRewards = activity.DefaultScratchRewards()

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
	if err := serve(port, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
		os.Exit(1)
	}
}

func serve(port string, handler http.Handler) error {
	return newHTTPServer(port, handler).ListenAndServe()
}

func newHTTPServer(port string, handler http.Handler) *http.Server {
	return webutil.NewHTTPServer("0.0.0.0:"+port, handler)
}
func initAll() error {
	tokenSvc = nil
	tokenConfigErr = nil
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
	setMCYCookie(config.Env("MCY_COOKIE"))
	return nil
}

type spinResult struct {
	Type    string
	Dollars int
}

type httpErr struct {
	Status  int
	Message string
}
