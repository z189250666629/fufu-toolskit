package dragonboatcore

import (
	"fufu/prizepoolcore"
	"testing"
)

func TestFishConsumesFishingAttemptAndAddsZongziOnHit(t *testing.T) {
	round := NewRound("dragon-key")
	got := Fish(3, round, CastResult{Hit: true})
	if !got.Hit || got.FishingUsed != 1 || got.ZongziCaught != 1 || got.ZongziPeeled != 0 || got.Status != StatusCaught {
		t.Fatalf("hit fish = %#v", got)
	}

	got = Fish(3, Round{FishingUsed: 1, ZongziCaught: 1, Status: StatusCaught}, CastResult{Hit: false})
	if got.Hit || got.FishingUsed != 2 || got.ZongziCaught != 1 || got.Status != StatusCaught {
		t.Fatalf("miss fish should keep caught status while unpeeled zongzi exists: %#v", got)
	}
}

func TestFishStopsAtTotalDraws(t *testing.T) {
	got := Fish(2, Round{FishingUsed: 2, ZongziCaught: 1, ZongziPeeled: 1, Status: StatusPeeled}, CastResult{Hit: true})
	if got.Hit || got.FishingUsed != 2 || got.ZongziCaught != 1 || got.Status != StatusPeeled {
		t.Fatalf("exhausted fish = %#v", got)
	}
}

func TestCalculateSceneSeedBuildsStableRichScene(t *testing.T) {
	input := SceneSeedInput{CardKey: "sk-dragon-scene", TotalDraws: 10, FishingUsed: 3, ZongziCaught: 2, ZongziPeeled: 1}
	first := CalculateSceneSeed(input)
	second := CalculateSceneSeed(input)

	if first.Seed == "" || first.Seed != second.Seed {
		t.Fatalf("scene seed should be stable and non-empty: %#v %#v", first, second)
	}
	if first.Attempt != 4 || first.ObstacleCount != len(first.Obstacles) || first.ZongziCount != len(first.Zongzi) {
		t.Fatalf("scene metadata wrong: %#v", first)
	}
	if first.ObstacleCount < 7 || first.ZongziCount < 9 {
		t.Fatalf("scene should be richer than the old 3/3 layout: obstacles=%d zongzi=%d", first.ObstacleCount, first.ZongziCount)
	}
	for _, item := range append(first.Obstacles, first.Zongzi...) {
		if item.ID == "" || item.X < 6 || item.X > 94 || item.Y < 28 || item.Y > 92 || item.Radius <= 0 {
			t.Fatalf("scene item out of playable bounds: %#v", item)
		}
	}

	nextAttempt := CalculateSceneSeed(SceneSeedInput{CardKey: "sk-dragon-scene", TotalDraws: 10, FishingUsed: 4, ZongziCaught: 2, ZongziPeeled: 1})
	if nextAttempt.Seed != first.Seed || nextAttempt.Obstacles[0] != first.Obstacles[0] || nextAttempt.Zongzi[0] != first.Zongzi[0] {
		t.Fatalf("fishing progress should not reshuffle scene coordinates: first=%#v next=%#v", first, nextAttempt)
	}
	if nextAttempt.Attempt != 5 {
		t.Fatalf("next fishing attempt should still update attempt metadata: %#v", nextAttempt)
	}
}

func TestCalculateSceneSeedFiltersPulledObjectsWithoutReshufflingRemainingScene(t *testing.T) {
	input := SceneSeedInput{CardKey: "sk-dragon-scene", TotalDraws: 10}
	full := CalculateSceneSeed(input)
	if len(full.Obstacles) == 0 || len(full.Zongzi) == 0 {
		t.Fatalf("scene should contain removable objects: %#v", full)
	}

	filtered := CalculateSceneSeed(SceneSeedInput{
		CardKey:          "sk-dragon-scene",
		TotalDraws:       10,
		FishingUsed:      1,
		RemovedObjectIDs: []string{full.Obstacles[0].ID, full.Zongzi[0].ID},
	})
	if filtered.Seed != full.Seed {
		t.Fatalf("removed objects should not change scene seed: full=%#v filtered=%#v", full, filtered)
	}
	if containsSceneObject(filtered.Obstacles, full.Obstacles[0].ID) || containsSceneObject(filtered.Zongzi, full.Zongzi[0].ID) {
		t.Fatalf("removed objects should be filtered from next scene: %#v", filtered)
	}
	if len(filtered.Obstacles) != len(full.Obstacles)-1 || len(filtered.Zongzi) != len(full.Zongzi)-1 {
		t.Fatalf("filtered scene counts wrong: full=%#v filtered=%#v", full, filtered)
	}
	if filtered.Obstacles[0] != full.Obstacles[1] || filtered.Zongzi[0] != full.Zongzi[1] {
		t.Fatalf("remaining objects should keep original coordinates: full=%#v filtered=%#v", full, filtered)
	}
}

func TestResolveCastHitsZongziOnlyWhenPathIsClear(t *testing.T) {
	scene := SceneSeed{
		Seed:       "manual",
		HookOrigin: Point{X: 50, Y: 18},
		Zongzi:     []SceneObject{{ID: "z1", X: 50, Y: 70, Radius: 6}},
		Obstacles:  []SceneObject{{ID: "o1", X: 22, Y: 55, Radius: 7}},
	}

	got := ResolveCast(scene, Point{X: 50, Y: 88})
	if !got.Hit || got.ZongziID != "z1" || got.BlockedBy != "" {
		t.Fatalf("clear cast should catch zongzi: %#v", got)
	}
}

func TestResolveCastMissesWhenObstacleBlocksBeforeZongzi(t *testing.T) {
	scene := SceneSeed{
		Seed:       "manual",
		HookOrigin: Point{X: 50, Y: 18},
		Zongzi:     []SceneObject{{ID: "z1", X: 50, Y: 70, Radius: 6}},
		Obstacles:  []SceneObject{{ID: "o1", X: 50, Y: 48, Radius: 7}},
	}

	got := ResolveCast(scene, Point{X: 50, Y: 88})
	if got.Hit || got.ZongziID != "" || got.BlockedBy != "o1" {
		t.Fatalf("blocked cast should miss with blocker id: %#v", got)
	}
}

func TestResolveCastReturnsObstacleWhenNoZongziIsOnPath(t *testing.T) {
	scene := SceneSeed{
		Seed:       "manual",
		HookOrigin: Point{X: 50, Y: 18},
		Zongzi:     []SceneObject{{ID: "z1", X: 80, Y: 72, Radius: 6}},
		Obstacles:  []SceneObject{{ID: "o1", X: 50, Y: 48, Radius: 7}},
	}

	got := ResolveCast(scene, Point{X: 50, Y: 88})
	if got.Hit || got.ZongziID != "" || got.BlockedBy != "o1" {
		t.Fatalf("obstacle-only cast should pull blocker and miss: %#v", got)
	}
}

func TestPeelRollsPrizeAndAdvancesPeeledCount(t *testing.T) {
	pool := []prizepoolcore.Prize{
		{Type: "miss", Weight: 1},
		{Type: "win", Dollars: 8, Weight: 1},
	}
	got, ok := Peel(Config{PrizePool: pool}, 2, Round{FishingUsed: 1, ZongziCaught: 1, ZongziPeeled: 0, Status: StatusCaught}, func(max int) int { return 1 })
	if !ok || got.Prize.Dollars != 8 || got.ZongziPeeled != 1 || got.Status != StatusFishing {
		t.Fatalf("peel = %#v ok=%v", got, ok)
	}
}

func TestPeelFinishesWhenAllFishingAndCaughtZongziAreDone(t *testing.T) {
	got, ok := Peel(Config{PrizePool: []prizepoolcore.Prize{{Type: "miss", Weight: 1}}}, 1, Round{FishingUsed: 1, ZongziCaught: 1, ZongziPeeled: 0, Status: StatusCaught}, func(max int) int { return 0 })
	if !ok || got.Status != StatusPeeled {
		t.Fatalf("final peel = %#v ok=%v", got, ok)
	}
}

func TestPeelRejectsWhenNothingCaught(t *testing.T) {
	got, ok := Peel(Config{}, 2, Round{FishingUsed: 1, ZongziCaught: 0, ZongziPeeled: 0, Status: StatusFishing}, func(max int) int { return 0 })
	if ok || got.Status != StatusFishing {
		t.Fatalf("empty peel = %#v ok=%v", got, ok)
	}
}

func containsSceneObject(items []SceneObject, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
