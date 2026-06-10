package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondCardReturns500WhenHistoryQueryFails(t *testing.T) {
	setupScratchLockTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if x := recover(); x != nil {
			t.Fatalf("respondCard should return 500, not panic: %v", x)
		}
	}()
	w := httptest.NewRecorder()
	respondCard(w, Card{CardKey: "broken-card", CardName: "broken", Dollars: 100, TotalSpins: 1})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestRespondCardPreservesHistoryAndSpinPayload(t *testing.T) {
	setupScratchLockTestDB(t)
	for _, row := range []struct {
		prize int
		retry int
	}{
		{prize: 10, retry: 0},
		{prize: 0, retry: 0},
		{prize: 20, retry: 0},
		{prize: 30, retry: 1},
	} {
		if _, err := db.Exec(`INSERT INTO spin_log (card_key, prize_dollars, is_retry) VALUES (?,?,?)`, "spin-card", row.prize, row.retry); err != nil {
			t.Fatal(err)
		}
	}

	w := httptest.NewRecorder()
	respondCard(w, Card{CardKey: "spin-card", CardName: "Spin Card", Dollars: 100, TotalSpins: 3, UsedSpins: 1, TotalWon: 30})

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		CardKey        string           `json:"cardKey"`
		CardName       string           `json:"cardName"`
		RemainingSpins int              `json:"remainingSpins"`
		TotalWon       int              `json:"totalWon"`
		IsScratch      bool             `json:"isScratch"`
		History        []map[string]any `json:"history"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.CardKey != "spin-card" || body.CardName != "Spin Card" || body.RemainingSpins != 2 || body.TotalWon != 30 || body.IsScratch {
		t.Fatalf("unexpected response: %#v body=%s", body, w.Body.String())
	}
	if len(body.History) != 2 || body.History[0]["prize_dollars"].(float64) != 20 || body.History[1]["prize_dollars"].(float64) != 10 {
		t.Fatalf("history = %#v; body=%s", body.History, w.Body.String())
	}
}

func TestRespondCardRequiresExactScratchDollarTier(t *testing.T) {
	setupScratchLockTestDB(t)

	w := httptest.NewRecorder()
	respondCard(w, Card{CardKey: "near-scratch-card", CardName: "Near Scratch", Dollars: 55.4, TotalSpins: 0})

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		IsScratch bool `json:"isScratch"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.IsScratch {
		t.Fatalf("near-55 card should not be marked scratch: body=%s", w.Body.String())
	}
}

func TestRespondCardReturns500WhenScratchLookupFails(t *testing.T) {
	setupScratchLockTestDB(t)
	if _, err := db.Exec(`DROP TABLE scratch_games`); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	respondCard(w, Card{CardKey: "scratch-card", CardName: "Scratch Card", Dollars: 55, TotalSpins: 0})

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
