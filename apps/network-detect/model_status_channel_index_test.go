package main

import (
	"strings"
	"testing"
)

func TestIndexChannelsForModelStatusDedupesGroupsAndIndexesModelGroups(t *testing.T) {
	index := indexChannelsForModelStatus([]Channel{
		{ID: 1, Status: channelStatusEnabled, Models: []string{"model-a"}, Groups: []string{"vip", "default"}},
		{ID: 2, Status: 2, Models: []string{"model-a", "model-b"}, Groups: []string{"vip"}},
		{ID: 3, Status: channelStatusEnabled, Models: []string{"model-b"}, Groups: []string{}},
	})

	if got := strings.Join(index.groups, ","); got != "default,vip" {
		t.Fatalf("groups = %#v", index.groups)
	}
	if got := len(index.channelsByModel["model-a"]); got != 2 {
		t.Fatalf("model-a channel count = %d", got)
	}
	if got := len(index.channelsByModel["model-b"]); got != 2 {
		t.Fatalf("model-b channel count = %d", got)
	}
	if index.channelsByModel["model-a"][1].ID != 2 {
		t.Fatalf("disabled channel should remain indexed for total count: %#v", index.channelsByModel["model-a"])
	}
}
