package combine

import "testing"

func TestBuildNewMergeTokenBodyUsesTargetPlan(t *testing.T) {
	body := buildNewMergeTokenBody("merge-temp", mergeTargetPlan{Quota: 123, Group: "vip"}, 60)

	if body["name"] != "merge-temp" || body["remain_quota"] != int64(123) || body["interval_quota"] != int64(123) {
		t.Fatalf("quota/name fields = %#v", body)
	}
	if body["group"] != "vip" || body["interval_unit"] != 60 || body["unlimited_quota"] != false {
		t.Fatalf("settings fields = %#v", body)
	}
	if body["expired_time"] != -1 || body["interval_time"] != -1 || body["trigger_last_time"] != 0 {
		t.Fatalf("time fields = %#v", body)
	}
}

func TestBuildMergeResultKeepsFullKeyAndFallsBackToPlan(t *testing.T) {
	deleteResults := []DeleteResult{{ID: 1, Key: "sk-old", OK: true}}
	result := buildMergeResult(
		map[string]any{"key": "new", "name": "final", "remain_quota": 0, "interval_unit": 0, "group": ""},
		mergeTargetPlan{Quota: 456, Group: "fallback"},
		30,
		deleteResults,
	)

	if !result.Success || result.NewCard.Key != "sk-new" || result.NewCard.Name != "final" {
		t.Fatalf("new card identity = %#v", result.NewCard)
	}
	if result.NewCard.RemainQuota != 456 || result.NewCard.IntervalUnit != 30 || result.NewCard.Group != "fallback" {
		t.Fatalf("fallback fields = %#v", result.NewCard)
	}
	if len(result.DeleteResults) != 1 || result.DeleteResults[0].ID != 1 {
		t.Fatalf("delete results = %#v", result.DeleteResults)
	}
}
