package main

import "testing"

func TestSelectModelTestChannelsFiltersAndSorts(t *testing.T) {
	channels := []Channel{
		{ID: 1, Status: channelStatusEnabled, Models: []string{"model-a"}, Groups: []string{"default"}, ResponseTime: 300},
		{ID: 2, Status: channelStatusEnabled, Models: []string{"model-a"}, Groups: []string{"vip"}, ResponseTime: 100},
		{ID: 3, Status: 2, Models: []string{"model-a"}, Groups: []string{"vip"}, ResponseTime: 10},
		{ID: 4, Status: channelStatusEnabled, Models: []string{"model-b"}, Groups: []string{"vip"}, ResponseTime: 20},
	}

	got := selectModelTestChannels(channels, "model-a", "vip")
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("vip candidates = %#v", got)
	}

	got = selectModelTestChannels(channels, "model-a", "")
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 1 {
		t.Fatalf("all group candidates = %#v", got)
	}
}

func TestChannelTestEndpointEscapesModelAndStreamFlag(t *testing.T) {
	if got := channelTestEndpoint(7, "gpt model", true); got != "/api/channel/test/7?model=gpt+model&stream=true" {
		t.Fatalf("stream endpoint = %q", got)
	}
	if got := channelTestEndpoint(7, "gpt model", false); got != "/api/channel/test/7?model=gpt+model" {
		t.Fatalf("endpoint = %q", got)
	}
}
