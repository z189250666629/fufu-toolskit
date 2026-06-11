package activityapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScratchStartIncludesMinesForFinishedGame(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchCard(t, "scratch-card")
	seedScratchGame(t, "scratch-card", "[1,2]", "[0]", 0, "lost")

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/start", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchStart(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Mines []int `json:"mines"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Mines) != 2 || body.Mines[0] != 1 || body.Mines[1] != 2 {
		t.Fatalf("mines = %#v; body=%s", body.Mines, w.Body.String())
	}
}
