package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

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
	if err := run(wd, os.Getenv("SLOT_PORT")); err != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
		os.Exit(1)
	}
}

func run(wd, portValue string) error {
	port, err := webutil.ResolvePort(portValue, defaultPort)
	if err != nil {
		return fmt.Errorf("invalid SLOT_PORT: %w", err)
	}
	rootDir = wd
	if err := initAll(); err != nil {
		return err
	}
	if db != nil {
		defer db.Close()
	}
	go creditWorker()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", apiRoute)
	mux.HandleFunc("/", staticRoute)
	fmt.Printf("fufu-act Go backend listening on :%s\n", port)
	return serve(port, mux)
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
