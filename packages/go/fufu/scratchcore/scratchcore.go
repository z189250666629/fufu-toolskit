package scratchcore

import (
	"encoding/json"
	"errors"
)

var ErrInvalidCells = errors.New("invalid scratch cells")
var ErrInvalidCell = errors.New("invalid scratch cell")
var ErrCellAlreadyRevealed = errors.New("scratch cell already revealed")
var ErrGameNotPlaying = errors.New("scratch game not playing")
var ErrNoSafeCell = errors.New("scratch cashout requires safe cell")

type Game struct {
	MineCells []int
	Revealed  []int
	Prize     int
	Status    string
}

type RevealResult struct {
	Hit      bool
	Mines    []int
	Prize    int
	Status   string
	Revealed []int
}

type CashoutResult struct {
	Mines    []int
	Prize    int
	Status   string
	Revealed []int
}

func ParseIntArray(s string) ([]int, error) {
	var cells []int
	if err := json.Unmarshal([]byte(s), &cells); err != nil {
		return nil, err
	}
	if cells == nil {
		return []int{}, nil
	}
	return cells, nil
}

func ParseRevealedCells(s string, maxReveals, cellCount int) ([]int, error) {
	cells, err := ParseIntArray(s)
	if err != nil {
		return nil, err
	}
	if !ValidCells(cells, maxReveals, cellCount) {
		return nil, ErrInvalidCells
	}
	return cells, nil
}

func ParseMineCells(s string, mineCount, cellCount int) ([]int, error) {
	cells, err := ParseIntArray(s)
	if err != nil {
		return nil, err
	}
	if len(cells) != mineCount || !ValidCells(cells, mineCount, cellCount) {
		return nil, ErrInvalidCells
	}
	return cells, nil
}

func ValidCells(cells []int, maxCount, cellCount int) bool {
	if cellCount <= 0 {
		return false
	}
	if len(cells) > maxCount {
		return false
	}
	seen := map[int]bool{}
	for _, cell := range cells {
		if cell < 0 || cell >= cellCount || seen[cell] {
			return false
		}
		seen[cell] = true
	}
	return true
}

func Contains(cells []int, value int) bool {
	for _, cell := range cells {
		if cell == value {
			return true
		}
	}
	return false
}

func SafeCount(revealed, mines []int) int {
	safe := 0
	for _, cell := range revealed {
		if !Contains(mines, cell) {
			safe++
		}
	}
	return safe
}

func PrizeForSafeCount(rewards []int, safe int) (int, bool) {
	if safe <= 0 || safe > len(rewards) {
		return 0, false
	}
	return rewards[safe-1], true
}

func Reveal(game Game, cellIndex int, rewards []int, maxReveals, mineCount, cellCount int) (RevealResult, error) {
	if game.Status != "playing" {
		return RevealResult{}, ErrGameNotPlaying
	}
	if cellIndex < 0 || cellIndex >= cellCount {
		return RevealResult{}, ErrInvalidCell
	}
	if len(game.MineCells) != mineCount || !ValidCells(game.MineCells, mineCount, cellCount) || !ValidCells(game.Revealed, maxReveals, cellCount) {
		return RevealResult{}, ErrInvalidCells
	}
	if Contains(game.Revealed, cellIndex) {
		return RevealResult{}, ErrCellAlreadyRevealed
	}

	revealed := append(cloneInts(game.Revealed), cellIndex)
	if Contains(game.MineCells, cellIndex) {
		return RevealResult{
			Hit:      true,
			Mines:    cloneInts(game.MineCells),
			Prize:    0,
			Status:   "lost",
			Revealed: revealed,
		}, nil
	}

	safe := SafeCount(revealed, game.MineCells)
	prize, ok := PrizeForSafeCount(rewards, safe)
	if !ok {
		return RevealResult{}, ErrInvalidCells
	}
	status := "playing"
	var mines []int
	if safe >= maxReveals {
		status = "won"
		mines = cloneInts(game.MineCells)
	}
	return RevealResult{
		Hit:      false,
		Mines:    mines,
		Prize:    prize,
		Status:   status,
		Revealed: revealed,
	}, nil
}

func Cashout(game Game, rewards []int, maxReveals, mineCount, cellCount int) (CashoutResult, error) {
	if game.Status != "playing" {
		return CashoutResult{}, ErrGameNotPlaying
	}
	if len(game.MineCells) != mineCount || !ValidCells(game.MineCells, mineCount, cellCount) || !ValidCells(game.Revealed, maxReveals, cellCount) {
		return CashoutResult{}, ErrInvalidCells
	}
	safe := SafeCount(game.Revealed, game.MineCells)
	if safe == 0 {
		return CashoutResult{}, ErrNoSafeCell
	}
	prize, ok := PrizeForSafeCount(rewards, safe)
	if !ok {
		return CashoutResult{}, ErrInvalidCells
	}
	return CashoutResult{
		Mines:    cloneInts(game.MineCells),
		Prize:    prize,
		Status:   "cashout",
		Revealed: cloneInts(game.Revealed),
	}, nil
}

func CanStartRound(total, used int) bool {
	return total <= 0 || used < total
}

func IsGameOver(status string) bool {
	return status == "won" || status == "lost" || status == "cashout"
}

func GameResponse(minePos, revealedRaw string, prize int, status string, maxReveals, mineCount, cellCount int) (map[string]any, error) {
	revealed, err := ParseRevealedCells(revealedRaw, maxReveals, cellCount)
	if err != nil {
		return nil, err
	}
	response := map[string]any{"revealed": revealed, "prize": prize, "status": status}
	if IsGameOver(status) {
		mines, err := ParseMineCells(minePos, mineCount, cellCount)
		if err != nil {
			return nil, err
		}
		response["mines"] = mines
	}
	return response, nil
}

func StartResponse(minePos, revealedRaw string, prize int, status string, maxReveals, mineCount, cellCount int) (map[string]any, error) {
	response, err := GameResponse(minePos, revealedRaw, prize, status, maxReveals, mineCount, cellCount)
	if err != nil {
		return nil, err
	}
	response["cells"] = cellCount
	return response, nil
}

func cloneInts(values []int) []int {
	return append([]int(nil), values...)
}
