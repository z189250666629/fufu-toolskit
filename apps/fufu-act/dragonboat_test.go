package activityapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fufu/activity"
	"fufu/dragonboatcore"
)

func setDragonBoatRuntimeConfig(t *testing.T, drawCount int, prizePool []activity.Prize) {
	t.Helper()
	original := SnapshotRuntimeConfig()
	t.Cleanup(func() { SetRuntimeConfig(original) })

	cfg := activity.DefaultConfig()
	cfg.GameRoutes = []activity.GameRoute{{Dollars: 55, Game: activity.GameDragon, DrawCount: drawCount}}
	cfg.PrizePool = prizePool
	cfg.GameConfigs = []activity.GameConfig{
		{Game: activity.GameSlot, TargetExpectedValue: activity.ExpectedValue(prizePool), ActualExpectedValue: activity.ExpectedValue(prizePool)},
		{Game: activity.GameScratch, TargetExpectedValue: activity.ExpectedValue(prizePool), ActualExpectedValue: activity.ExpectedValue(prizePool)},
		{Game: activity.GameDragon, TargetExpectedValue: activity.ExpectedValue(prizePool), ActualExpectedValue: activity.ExpectedValue(prizePool)},
	}
	SetRuntimeConfig(cfg)
}

func seedDragonBoatCard(t *testing.T, key string, dollars float64, draws int) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`,
		key,
		"dragonboat-test",
		dollars,
		draws,
	); err != nil {
		t.Fatal(err)
	}
}

func postDragonBoat(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	switch path {
	case "/api/dragonboat/start":
		handleDragonBoatStart(w, req)
	case "/api/dragonboat/fish":
		handleDragonBoatFish(w, req)
	case "/api/dragonboat/peel":
		handleDragonBoatPeel(w, req)
	default:
		t.Fatalf("unsupported path %s", path)
	}
	return w
}

func decodeDragonBoatResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON body: %s err=%v", rec.Body.String(), err)
	}
	return out
}

func TestDragonBoatStartAndFishUseConfiguredFishingCount(t *testing.T) {
	setupScratchLockTestDB(t)
	setDragonBoatRuntimeConfig(t, 2, []activity.Prize{{Type: "win", Dollars: 8, Weight: 1}})
	key := "sk-dragon-fish"
	seedDragonBoatCard(t, key, 55, 2)

	start := postDragonBoat(t, "/api/dragonboat/start", `{"cardKey":"sk-dragon-fish"}`)
	if start.Code != http.StatusOK {
		t.Fatalf("start code=%d body=%s", start.Code, start.Body.String())
	}
	startBody := decodeDragonBoatResponse(t, start)
	if startBody["fishingTotal"] != float64(2) || startBody["remainingFish"] != float64(2) {
		t.Fatalf("start body=%#v", startBody)
	}
	if startBody["sceneSeed"] == nil {
		t.Fatalf("start body should include scene seed: %#v", startBody)
	}
	startScene := sceneSeedFromResponse(t, startBody)
	hitTarget, ok := findDragonBoatTarget(t, dragonBoatSceneSeed(DragonBoatGame{CardKey: key}, Card{CardKey: key, TotalSpins: 2}), true)
	if !ok {
		t.Fatal("test scene should contain at least one reachable zongzi target")
	}

	first := postDragonBoat(t, "/api/dragonboat/fish", dragonBoatFishBody(key, hitTarget))
	if first.Code != http.StatusOK {
		t.Fatalf("first fish code=%d body=%s", first.Code, first.Body.String())
	}
	firstBody := decodeDragonBoatResponse(t, first)
	if firstBody["hit"] != true || firstBody["fishingUsed"] != float64(1) || firstBody["remainingFish"] != float64(1) || firstBody["zongziReady"] != float64(1) {
		t.Fatalf("first fish body=%#v", firstBody)
	}
	cast := firstBody["cast"].(map[string]any)
	if cast["hit"] != true || cast["zongziId"] == "" {
		t.Fatalf("first fish should report a successful cast: %#v", cast)
	}
	firstScene := sceneSeedFromResponse(t, firstBody)
	zongziID := cast["zongziId"].(string)
	if firstScene["seed"] != startScene["seed"] {
		t.Fatalf("fishing should not reshuffle scene seed: start=%#v first=%#v", startScene, firstScene)
	}
	if sceneHasObject(firstScene["zongzi"], zongziID) {
		t.Fatalf("caught zongzi %q should be removed from next scene: %#v", zongziID, firstScene["zongzi"])
	}

	nextGame, ok, err := lookupDragonBoat(key)
	if err != nil || !ok {
		t.Fatalf("lookup next dragonboat game ok=%v err=%v", ok, err)
	}
	nextScene := dragonBoatSceneSeed(nextGame, Card{CardKey: key, TotalSpins: 2})
	missTarget, ok := findDragonBoatTarget(t, nextScene, false)
	if !ok {
		t.Fatal("test scene should contain at least one miss target")
	}
	second := postDragonBoat(t, "/api/dragonboat/fish", dragonBoatFishBody(key, missTarget))
	if second.Code != http.StatusOK {
		t.Fatalf("second fish code=%d body=%s", second.Code, second.Body.String())
	}
	secondBody := decodeDragonBoatResponse(t, second)
	if secondBody["hit"] != false || secondBody["fishingUsed"] != float64(2) || secondBody["remainingFish"] != float64(0) || secondBody["zongziReady"] != float64(1) {
		t.Fatalf("second fish body=%#v", secondBody)
	}

	var usedSpins, totalWon, logs int
	if err := db.QueryRow(`SELECT used_spins,total_won FROM cards WHERE card_key=?`, key).Scan(&usedSpins, &totalWon); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM spin_log WHERE card_key=?`, key).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if usedSpins != 2 || totalWon != 0 || logs != 0 {
		t.Fatalf("fish should consume fishing attempts without prize log: used=%d won=%d logs=%d", usedSpins, totalWon, logs)
	}
}

func TestDragonBoatFishBlockedByObstacleConsumesAttemptWithoutAddingZongzi(t *testing.T) {
	setupScratchLockTestDB(t)
	setDragonBoatRuntimeConfig(t, 2, []activity.Prize{{Type: "win", Dollars: 8, Weight: 1}})
	key := "sk-dragon-blocked"
	seedDragonBoatCard(t, key, 55, 2)

	scene := dragonBoatSceneSeed(DragonBoatGame{CardKey: key}, Card{CardKey: key, TotalSpins: 2})
	blockedTarget, ok := findDragonBoatBlockedTarget(t, scene)
	if !ok {
		t.Fatal("test scene should contain at least one obstacle-blocked target")
	}

	rec := postDragonBoat(t, "/api/dragonboat/fish", dragonBoatFishBody(key, blockedTarget))
	if rec.Code != http.StatusOK {
		t.Fatalf("fish code=%d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeDragonBoatResponse(t, rec)
	if body["hit"] != false || body["fishingUsed"] != float64(1) || body["remainingFish"] != float64(1) || body["zongziReady"] != float64(0) {
		t.Fatalf("blocked fish should consume one attempt without adding zongzi: %#v", body)
	}
	cast := body["cast"].(map[string]any)
	if cast["hit"] != false || cast["blockedBy"] == "" {
		t.Fatalf("blocked fish should report blocker and miss: %#v", cast)
	}
	if cast["zongziId"] != nil && cast["zongziId"] != "" {
		t.Fatalf("blocked fish should report only one pulled object, got cast=%#v", cast)
	}
	nextScene := sceneSeedFromResponse(t, body)
	blockedBy := cast["blockedBy"].(string)
	if sceneHasObject(nextScene["obstacles"], blockedBy) {
		t.Fatalf("blocked obstacle %q should be removed from next scene: %#v", blockedBy, nextScene["obstacles"])
	}

	var usedSpins, totalWon, logs int
	if err := db.QueryRow(`SELECT used_spins,total_won FROM cards WHERE card_key=?`, key).Scan(&usedSpins, &totalWon); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM spin_log WHERE card_key=?`, key).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if usedSpins != 1 || totalWon != 0 || logs != 0 {
		t.Fatalf("blocked fish should not roll prizes: used=%d won=%d logs=%d", usedSpins, totalWon, logs)
	}
}

func TestDragonBoatPeelRecordsPrizeAndQueuesAfterAllFishingIsResolved(t *testing.T) {
	setupScratchLockTestDB(t)
	setDragonBoatRuntimeConfig(t, 2, []activity.Prize{{Type: "win", Dollars: 8, Weight: 1}})
	useRandomSequence(t, 0)
	key := "sk-dragon-peel-final"
	seedDragonBoatCard(t, key, 55, 2)
	if _, err := db.Exec(`UPDATE cards SET used_spins=2 WHERE card_key=?`, key); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO dragonboat_games (card_key,fishing_used,zongzi_caught,zongzi_peeled,status) VALUES (?,?,?,?,?)`, key, 2, 1, 0, "caught"); err != nil {
		t.Fatal(err)
	}

	rec := postDragonBoat(t, "/api/dragonboat/peel", `{"cardKey":"sk-dragon-peel-final"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("peel code=%d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeDragonBoatResponse(t, rec)
	prize := body["prizeResult"].(map[string]any)
	if body["status"] != "peeled" || body["totalWon"] != float64(8) || prize["prize"] != float64(8) {
		t.Fatalf("peel body=%#v", body)
	}

	var usedSpins, totalWon, logs, queuedPrize int
	if err := db.QueryRow(`SELECT used_spins,total_won FROM cards WHERE card_key=?`, key).Scan(&usedSpins, &totalWon); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM spin_log WHERE card_key=?`, key).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT prize_dollars FROM credit_queue WHERE card_key=?`, key).Scan(&queuedPrize); err != nil {
		t.Fatal(err)
	}
	if usedSpins != 2 || totalWon != 8 || logs != 1 || queuedPrize != 8 {
		t.Fatalf("final peel should log prize and enqueue total prize: used=%d won=%d logs=%d queued=%d", usedSpins, totalWon, logs, queuedPrize)
	}
}

func TestDragonBoatPeelBeforeFishingEndsDoesNotQueueCredit(t *testing.T) {
	setupScratchLockTestDB(t)
	setDragonBoatRuntimeConfig(t, 2, []activity.Prize{{Type: "win", Dollars: 8, Weight: 1}})
	useRandomSequence(t, 0)
	key := "sk-dragon-peel-open"
	seedDragonBoatCard(t, key, 55, 2)
	if _, err := db.Exec(`UPDATE cards SET used_spins=1 WHERE card_key=?`, key); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO dragonboat_games (card_key,fishing_used,zongzi_caught,zongzi_peeled,status) VALUES (?,?,?,?,?)`, key, 1, 1, 0, "caught"); err != nil {
		t.Fatal(err)
	}

	rec := postDragonBoat(t, "/api/dragonboat/peel", `{"cardKey":"sk-dragon-peel-open"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("peel code=%d body=%s", rec.Code, rec.Body.String())
	}

	var usedSpins, totalWon, queued int
	if err := db.QueryRow(`SELECT used_spins,total_won FROM cards WHERE card_key=?`, key).Scan(&usedSpins, &totalWon); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM credit_queue WHERE card_key=?`, key).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if usedSpins != 1 || totalWon != 8 || queued != 0 {
		t.Fatalf("open peel should keep fishing progress and not queue: used=%d won=%d queued=%d", usedSpins, totalWon, queued)
	}
}

func useRandomSequence(t *testing.T, values ...int) {
	t.Helper()
	old := secureRandomInt
	index := 0
	secureRandomInt = func(max int) int {
		if index >= len(values) {
			return 0
		}
		value := values[index]
		index++
		if max <= 0 {
			return 0
		}
		if value < 0 {
			return 0
		}
		return value % max
	}
	t.Cleanup(func() { secureRandomInt = old })
}

func dragonBoatFishBody(key string, target dragonboatcore.Point) string {
	data, _ := json.Marshal(map[string]any{
		"cardKey": key,
		"target":  target,
	})
	return string(data)
}

func findDragonBoatTarget(t *testing.T, scene dragonboatcore.SceneSeed, wantHit bool) (dragonboatcore.Point, bool) {
	t.Helper()
	for _, item := range scene.Zongzi {
		target := dragonboatcore.Point{X: item.X, Y: item.Y}
		if dragonboatcore.ResolveCast(scene, target).Hit == wantHit {
			return target, true
		}
	}
	for x := 6.0; x <= 94; x += 8 {
		for y := 30.0; y <= 94; y += 8 {
			target := dragonboatcore.Point{X: x, Y: y}
			if dragonboatcore.ResolveCast(scene, target).Hit == wantHit {
				return target, true
			}
		}
	}
	return dragonboatcore.Point{}, false
}

func findDragonBoatBlockedTarget(t *testing.T, scene dragonboatcore.SceneSeed) (dragonboatcore.Point, bool) {
	t.Helper()
	for _, item := range scene.Obstacles {
		target := dragonboatcore.Point{X: item.X, Y: item.Y}
		if cast := dragonboatcore.ResolveCast(scene, target); !cast.Hit && cast.BlockedBy != "" {
			return target, true
		}
	}
	for x := 6.0; x <= 94; x += 4 {
		for y := 30.0; y <= 94; y += 4 {
			target := dragonboatcore.Point{X: x, Y: y}
			if cast := dragonboatcore.ResolveCast(scene, target); !cast.Hit && cast.BlockedBy != "" {
				return target, true
			}
		}
	}
	return dragonboatcore.Point{}, false
}

func sceneSeedFromResponse(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	scene, ok := body["sceneSeed"].(map[string]any)
	if !ok {
		t.Fatalf("response should contain scene seed: %#v", body)
	}
	return scene
}

func sceneHasObject(raw any, id string) bool {
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		row, ok := item.(map[string]any)
		if ok && row["id"] == id {
			return true
		}
	}
	return false
}

func TestDragonBoatRejectsNonDragonTierWithoutCreatingGame(t *testing.T) {
	setupScratchLockTestDB(t)
	setDragonBoatRuntimeConfig(t, 2, []activity.Prize{{Type: "win", Dollars: 8, Weight: 1}})
	key := "sk-dragon-wrong-tier"
	seedDragonBoatCard(t, key, 100, 1)

	rec := postDragonBoat(t, "/api/dragonboat/start", `{"cardKey":"sk-dragon-wrong-tier"}`)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "不参与端午捕粽") {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok, err := lookupDragonBoat(key); err != nil || ok {
		t.Fatalf("wrong tier should not create game, ok=%v err=%v", ok, err)
	}
}

func TestDragonBoatRejectsCachedDisabledTokenWithoutMutatingState(t *testing.T) {
	setupScratchLockTestDB(t)
	setDragonBoatRuntimeConfig(t, 2, []activity.Prize{{Type: "win", Dollars: 8, Weight: 1}})
	key := "sk-dragon-disabled"
	seedDragonBoatCard(t, key, 55, 2)
	useTokenStatusServer(t, key, 2)

	rec := postDragonBoat(t, "/api/dragonboat/fish", `{"cardKey":"sk-dragon-disabled"}`)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "已被禁用") {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var usedSpins, logs int
	if err := db.QueryRow(`SELECT used_spins FROM cards WHERE card_key=?`, key).Scan(&usedSpins); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM spin_log WHERE card_key=?`, key).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if usedSpins != 0 || logs != 0 {
		t.Fatalf("disabled dragonboat token should not mutate state: used=%d logs=%d", usedSpins, logs)
	}
	if _, ok, err := lookupDragonBoat(key); err != nil || ok {
		t.Fatalf("disabled token should not create game, ok=%v err=%v", ok, err)
	}
}
