package dragonboatcore

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"

	"fufu/prizepoolcore"
)

const (
	GameMode      = "dragonboat"
	StatusFishing = "fishing"
	StatusCaught  = "caught"
	StatusPeeled  = "peeled"
)

type Config struct {
	PrizePool []prizepoolcore.Prize
}

type Round struct {
	CardKey      string
	FishingUsed  int
	ZongziCaught int
	ZongziPeeled int
	Status       string
}

type CatchResult struct {
	Hit          bool
	Status       string
	FishingUsed  int
	ZongziCaught int
	ZongziPeeled int
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type SceneObject struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
	Scale  float64 `json:"scale"`
}

type SceneSeedInput struct {
	CardKey          string
	TotalDraws       int
	FishingUsed      int
	ZongziCaught     int
	ZongziPeeled     int
	RemovedObjectIDs []string
}

type SceneSeed struct {
	Seed          string        `json:"seed"`
	Attempt       int           `json:"attempt"`
	HookOrigin    Point         `json:"hookOrigin"`
	ObstacleCount int           `json:"obstacleCount"`
	ZongziCount   int           `json:"zongziCount"`
	Obstacles     []SceneObject `json:"obstacles"`
	Zongzi        []SceneObject `json:"zongzi"`
}

type CastResult struct {
	Hit       bool   `json:"hit"`
	Target    Point  `json:"target"`
	ZongziID  string `json:"zongziId,omitempty"`
	BlockedBy string `json:"blockedBy,omitempty"`
}

type PeelResult struct {
	Prize        prizepoolcore.Prize
	Status       string
	FishingUsed  int
	ZongziCaught int
	ZongziPeeled int
}

func NormalizeConfig(cfg Config) Config {
	cfg.PrizePool = append([]prizepoolcore.Prize(nil), cfg.PrizePool...)
	return cfg
}

func NewRound(cardKey string) Round {
	return Round{CardKey: cardKey, Status: StatusFishing}
}

func CanFish(totalDraws int, round Round) bool {
	return totalDraws > 0 && round.FishingUsed < totalDraws
}

func CanPeel(round Round) bool {
	return round.ZongziPeeled < round.ZongziCaught
}

func Fish(totalDraws int, round Round, cast CastResult) CatchResult {
	if !CanFish(totalDraws, round) {
		return CatchResult{
			Hit:          false,
			Status:       normalizeStatus(round),
			FishingUsed:  round.FishingUsed,
			ZongziCaught: round.ZongziCaught,
			ZongziPeeled: round.ZongziPeeled,
		}
	}
	round.FishingUsed++
	hit := cast.Hit
	if hit {
		round.ZongziCaught++
	}
	round.Status = statusFor(round, totalDraws)
	return CatchResult{
		Hit:          hit,
		Status:       round.Status,
		FishingUsed:  round.FishingUsed,
		ZongziCaught: round.ZongziCaught,
		ZongziPeeled: round.ZongziPeeled,
	}
}

func CalculateSceneSeed(input SceneSeedInput) SceneSeed {
	attempt := input.FishingUsed + 1
	if attempt < 1 {
		attempt = 1
	}
	if input.TotalDraws > 0 && attempt > input.TotalDraws {
		attempt = input.TotalDraws
	}
	raw := fmt.Sprintf("%s|%d", input.CardKey, input.TotalDraws)
	sum := sha256.Sum256([]byte(raw))
	seed := fmt.Sprintf("%x", sum[:8])
	rng := sceneRand{state: binary.LittleEndian.Uint64(sum[:8])}
	if rng.state == 0 {
		rng.state = 1
	}

	removed := removedObjectSet(input.RemovedObjectIDs)
	obstacleCount := 7 + positiveMod(input.TotalDraws, 3)
	zongziCount := 9 + positiveMod(input.TotalDraws, 4)
	scene := SceneSeed{
		Seed:       seed,
		Attempt:    attempt,
		HookOrigin: Point{X: 50, Y: 18},
		Obstacles:  make([]SceneObject, 0, obstacleCount),
		Zongzi:     make([]SceneObject, 0, zongziCount),
	}
	allObjects := make([]SceneObject, 0, obstacleCount+zongziCount)
	for i := 0; i < obstacleCount; i++ {
		item := SceneObject{
			ID:     fmt.Sprintf("o%d", i+1),
			X:      rng.rangeFloat(10, 90),
			Y:      rng.rangeFloat(36, 88),
			Radius: rng.rangeFloat(4.6, 6.9),
			Scale:  rng.rangeFloat(0.86, 1.28),
		}
		item = avoidCrowd(item, allObjects, 8, &rng)
		allObjects = append(allObjects, item)
		if !removed[item.ID] {
			scene.Obstacles = append(scene.Obstacles, item)
		}
	}
	for i := 0; i < zongziCount; i++ {
		item := SceneObject{
			ID:     fmt.Sprintf("z%d", i+1),
			X:      rng.rangeFloat(8, 92),
			Y:      rng.rangeFloat(34, 91),
			Radius: rng.rangeFloat(4.2, 5.8),
			Scale:  rng.rangeFloat(0.78, 1.12),
		}
		item = avoidCrowd(item, allObjects, 7, &rng)
		allObjects = append(allObjects, item)
		if !removed[item.ID] {
			scene.Zongzi = append(scene.Zongzi, item)
		}
	}
	scene.ObstacleCount = len(scene.Obstacles)
	scene.ZongziCount = len(scene.Zongzi)
	return scene
}

func ResolveCast(scene SceneSeed, target Point) CastResult {
	target = clampPoint(target)
	origin := scene.HookOrigin
	if origin.X == 0 && origin.Y == 0 {
		origin = Point{X: 50, Y: 18}
	}
	result := CastResult{Target: target}
	zongziID := ""
	zongziT := math.Inf(1)
	for _, item := range scene.Zongzi {
		if t, ok := segmentCircleParameter(origin, target, Point{X: item.X, Y: item.Y}, item.Radius); ok && t < zongziT {
			zongziT = t
			zongziID = item.ID
		}
	}

	blockedBy := ""
	blockT := math.Inf(1)
	for _, item := range scene.Obstacles {
		if t, ok := segmentCircleParameter(origin, target, Point{X: item.X, Y: item.Y}, item.Radius); ok && t < blockT {
			blockT = t
			blockedBy = item.ID
		}
	}
	if blockedBy != "" && blockT < zongziT {
		result.BlockedBy = blockedBy
		return result
	}
	if zongziID == "" {
		return result
	}
	result.ZongziID = zongziID
	result.Hit = true
	return result
}

func DefaultCastTarget(scene SceneSeed) Point {
	if len(scene.Zongzi) > 0 {
		return Point{X: scene.Zongzi[0].X, Y: scene.Zongzi[0].Y}
	}
	return Point{X: 50, Y: 82}
}

func Peel(cfg Config, totalDraws int, round Round, randomInt func(max int) int) (PeelResult, bool) {
	cfg = NormalizeConfig(cfg)
	if !CanPeel(round) {
		return PeelResult{
			Status:       normalizeStatus(round),
			FishingUsed:  round.FishingUsed,
			ZongziCaught: round.ZongziCaught,
			ZongziPeeled: round.ZongziPeeled,
		}, false
	}
	round.ZongziPeeled++
	round.Status = statusFor(round, totalDraws)
	return PeelResult{
		Prize:        prizepoolcore.Roll(cfg.PrizePool, randomInt),
		Status:       round.Status,
		FishingUsed:  round.FishingUsed,
		ZongziCaught: round.ZongziCaught,
		ZongziPeeled: round.ZongziPeeled,
	}, true
}

func statusFor(round Round, totalDraws int) string {
	if CanPeel(round) {
		return StatusCaught
	}
	if totalDraws > 0 && round.FishingUsed >= totalDraws {
		return StatusPeeled
	}
	return StatusFishing
}

func normalizeStatus(round Round) string {
	if round.Status != "" {
		return round.Status
	}
	return StatusFishing
}

type sceneRand struct {
	state uint64
}

func (r *sceneRand) next() uint64 {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return r.state
}

func (r *sceneRand) rangeFloat(min, max float64) float64 {
	v := float64(r.next()%10000) / 10000
	return min + (max-min)*v
}

func avoidCrowd(item SceneObject, existing []SceneObject, minDistance float64, rng *sceneRand) SceneObject {
	for attempt := 0; attempt < 10; attempt++ {
		crowded := false
		for _, other := range existing {
			if distance(Point{X: item.X, Y: item.Y}, Point{X: other.X, Y: other.Y}) < minDistance+item.Radius+other.Radius {
				crowded = true
				break
			}
		}
		if !crowded {
			return item
		}
		item.X = rng.rangeFloat(8, 92)
		item.Y = rng.rangeFloat(32, 92)
	}
	return item
}

func segmentCircleParameter(a, b, center Point, radius float64) (float64, bool) {
	ab := Point{X: b.X - a.X, Y: b.Y - a.Y}
	ac := Point{X: center.X - a.X, Y: center.Y - a.Y}
	abLen2 := ab.X*ab.X + ab.Y*ab.Y
	if abLen2 == 0 {
		return 0, distance(a, center) <= radius
	}
	t := (ac.X*ab.X + ac.Y*ab.Y) / abLen2
	if t < 0 || t > 1 {
		return 0, false
	}
	closest := Point{X: a.X + ab.X*t, Y: a.Y + ab.Y*t}
	if distance(closest, center) > radius {
		return 0, false
	}
	return t, true
}

func clampPoint(p Point) Point {
	if p.X < 0 {
		p.X = 0
	}
	if p.X > 100 {
		p.X = 100
	}
	if p.Y < 0 {
		p.Y = 0
	}
	if p.Y > 100 {
		p.Y = 100
	}
	return p
}

func distance(a, b Point) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

func positiveMod(value, mod int) int {
	if mod <= 0 {
		return 0
	}
	out := value % mod
	if out < 0 {
		out += mod
	}
	return out
}

func removedObjectSet(ids []string) map[string]bool {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" {
			out[id] = true
		}
	}
	return out
}
