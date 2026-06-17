package scratchcore

import (
	"errors"
	"testing"
)

func TestParseAndValidateCells(t *testing.T) {
	cells, err := ParseIntArray(`[1,2]`)
	if err != nil {
		t.Fatal(err)
	}
	if !Contains(cells, 2) || Contains(cells, 3) {
		t.Fatalf("parsed cells=%#v", cells)
	}
	if _, err := ParseIntArray(`not-json`); err == nil {
		t.Fatal("invalid JSON should fail")
	}
	if !ValidCells([]int{0, 8}, 2, 9) {
		t.Fatal("valid cells rejected")
	}
	for _, cells := range [][]int{{0, 0}, {-1}, {9}, {0, 1, 2}} {
		if ValidCells(cells, 2, 9) {
			t.Fatalf("invalid cells accepted: %#v", cells)
		}
	}
}

func TestGameResponseOnlyRevealsMinesAfterGameOver(t *testing.T) {
	playing, err := GameResponse("[7,8]", "[0]", 2, "playing", 6, 2, 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := playing["mines"]; ok {
		t.Fatalf("playing response should not reveal mines: %#v", playing)
	}
	lost, err := GameResponse("[7,8]", "[0,7]", 0, "lost", 6, 2, 9)
	if err != nil {
		t.Fatal(err)
	}
	if mines, ok := lost["mines"].([]int); !ok || len(mines) != 2 || mines[1] != 8 {
		t.Fatalf("lost response should include mines, got %#v", lost["mines"])
	}
	if _, err := GameResponse("[7,8]", "[0,1,2,3,4,5,6]", 0, "playing", 6, 2, 9); !errors.Is(err, ErrInvalidCells) {
		t.Fatalf("too many revealed cells should be invalid, got %v", err)
	}
}

func TestRevealAndPrizeRules(t *testing.T) {
	if safe := SafeCount([]int{0, 1, 7}, []int{7, 8}); safe != 2 {
		t.Fatalf("safe=%d, want 2", safe)
	}
	if prize, ok := PrizeForSafeCount([]int{2, 4, 6}, 2); !ok || prize != 4 {
		t.Fatalf("prize=%d ok=%v, want 4 true", prize, ok)
	}
	if _, ok := PrizeForSafeCount([]int{2, 4, 6}, 0); ok {
		t.Fatal("zero safe count should not have prize")
	}
	if !IsGameOver("won") || !IsGameOver("lost") || !IsGameOver("cashout") || IsGameOver("playing") {
		t.Fatal("game-over status classification is wrong")
	}
}

func TestRevealStateMachine(t *testing.T) {
	rewards := []int{2, 4, 6, 8, 12, 15}
	cases := []struct {
		name string
		game Game
		cell int
		want RevealResult
	}{
		{
			name: "safe cell keeps playing",
			game: Game{MineCells: []int{7, 8}, Revealed: []int{}, Status: "playing"},
			cell: 0,
			want: RevealResult{Hit: false, Prize: 2, Status: "playing", Revealed: []int{0}},
		},
		{
			name: "mine loses and exposes mines",
			game: Game{MineCells: []int{7, 8}, Revealed: []int{0}, Status: "playing"},
			cell: 7,
			want: RevealResult{Hit: true, Prize: 0, Status: "lost", Revealed: []int{0, 7}, Mines: []int{7, 8}},
		},
		{
			name: "sixth safe cell wins",
			game: Game{MineCells: []int{7, 8}, Revealed: []int{0, 1, 2, 3, 4}, Status: "playing"},
			cell: 5,
			want: RevealResult{Hit: false, Prize: 15, Status: "won", Revealed: []int{0, 1, 2, 3, 4, 5}, Mines: []int{7, 8}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Reveal(c.game, c.cell, rewards, 6, 2, 9)
			if err != nil {
				t.Fatalf("Reveal() error = %v", err)
			}
			if got.Hit != c.want.Hit || got.Prize != c.want.Prize || got.Status != c.want.Status || !sameInts(got.Revealed, c.want.Revealed) || !sameInts(got.Mines, c.want.Mines) {
				t.Fatalf("Reveal()=%#v, want %#v", got, c.want)
			}
		})
	}
}

func TestRevealRejectsInvalidState(t *testing.T) {
	rewards := []int{2, 4, 6, 8, 12, 15}
	cases := []struct {
		name string
		game Game
		cell int
		err  error
	}{
		{"not playing", Game{MineCells: []int{7, 8}, Status: "won"}, 0, ErrGameNotPlaying},
		{"duplicate cell", Game{MineCells: []int{7, 8}, Revealed: []int{0}, Status: "playing"}, 0, ErrCellAlreadyRevealed},
		{"invalid cell", Game{MineCells: []int{7, 8}, Status: "playing"}, 9, ErrInvalidCell},
		{"bad persisted mines", Game{MineCells: []int{7}, Status: "playing"}, 0, ErrInvalidCells},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Reveal(c.game, c.cell, rewards, 6, 2, 9)
			if !errors.Is(err, c.err) {
				t.Fatalf("Reveal() err=%v, want %v", err, c.err)
			}
		})
	}
}

func TestCashoutStateMachine(t *testing.T) {
	got, err := Cashout(Game{MineCells: []int{7, 8}, Revealed: []int{0, 1}, Status: "playing"}, []int{2, 4, 6}, 6, 2, 9)
	if err != nil {
		t.Fatalf("Cashout() error = %v", err)
	}
	if got.Prize != 4 || got.Status != "cashout" || !sameInts(got.Revealed, []int{0, 1}) || !sameInts(got.Mines, []int{7, 8}) {
		t.Fatalf("Cashout()=%#v", got)
	}

	if _, err := Cashout(Game{MineCells: []int{7, 8}, Revealed: []int{}, Status: "playing"}, []int{2, 4, 6}, 6, 2, 9); !errors.Is(err, ErrNoSafeCell) {
		t.Fatalf("cashout without safe cell err=%v", err)
	}
}

func TestCanStartRoundUsesCardDrawBudget(t *testing.T) {
	if !CanStartRound(0, 99) || !CanStartRound(2, 1) || CanStartRound(2, 2) {
		t.Fatal("CanStartRound should allow unlimited cards or unused configured rounds only")
	}
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
