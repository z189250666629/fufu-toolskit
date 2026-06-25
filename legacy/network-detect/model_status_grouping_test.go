package main

import "testing"

func TestModelGroupLogKeySeparatesModelAndGroup(t *testing.T) {
	if modelGroupLogKey("ab", "c") == modelGroupLogKey("a", "bc") {
		t.Fatalf("model/group keys should preserve tuple boundaries")
	}
}

func TestGroupLogsByModelAndModelGroup(t *testing.T) {
	rows := []LogRow{
		{ModelName: "model-a", Group: "vip default", RequestID: "req-1"},
		{ModelName: "model-a", Group: "vip", RequestID: "req-2"},
		{ModelName: "model-b", Group: "default", RequestID: "req-3"},
		{ModelName: "", Group: "vip", RequestID: "ignored-model"},
		{ModelName: "model-a", Group: "", RequestID: "ignored-group"},
	}

	byModel := groupLogs(rows)
	if got := len(byModel["model-a"]); got != 3 {
		t.Fatalf("model-a log count = %d", got)
	}
	if _, ok := byModel[""]; ok {
		t.Fatalf("blank model should not be grouped: %#v", byModel)
	}

	byModelGroup := groupLogsByModelGroup(rows)
	if got := len(byModelGroup[modelGroupLogKey("model-a", "vip")]); got != 2 {
		t.Fatalf("model-a/vip log count = %d", got)
	}
	if got := len(byModelGroup[modelGroupLogKey("model-a", "default")]); got != 1 {
		t.Fatalf("model-a/default log count = %d", got)
	}
	if got := len(byModelGroup[modelGroupLogKey("model-b", "default")]); got != 1 {
		t.Fatalf("model-b/default log count = %d", got)
	}
}
