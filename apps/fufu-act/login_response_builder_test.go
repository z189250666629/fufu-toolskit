package activityapp

import (
	"database/sql"
	"reflect"
	"testing"

	"fufu/activity"
)

func TestBuildLoginCardResponseCalculatesCardPayload(t *testing.T) {
	cfg := activity.DefaultConfig()
	cfg.ScratchTiers = []int{55}
	history := []map[string]any{{"prize_dollars": 20, "created_at": "2026-05-02 12:00:00"}}
	scratchGame := map[string]any{"status": "playing"}

	payload := buildLoginCardResponse(Card{
		CardKey:      "sk-response-card",
		CardName:     "response-card",
		Dollars:      55,
		TotalSpins:   3,
		UsedSpins:    1,
		WonJackpot:   1,
		TotalWon:     20,
		PurchaseTime: sql.NullString{String: "2026-05-02 12:00:00", Valid: true},
	}, history, scratchGame, cfg)

	if payload["cardKey"] != "sk-response-card" || payload["cardName"] != "response-card" {
		t.Fatalf("identity payload = %#v", payload)
	}
	if payload["remainingSpins"] != 2 || payload["totalWon"] != 20 || payload["wonJackpot"] != true {
		t.Fatalf("spin payload = %#v", payload)
	}
	if !reflect.DeepEqual(payload["history"], history) || payload["isScratch"] != true || !reflect.DeepEqual(payload["scratchGame"], scratchGame) {
		t.Fatalf("scratch payload = %#v", payload)
	}
}
