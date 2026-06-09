package main

import (
	"fmt"
	"net/url"
	"sort"
)

func selectModelTestChannels(channels []Channel, model, group string) []Channel {
	candidates := []Channel{}
	for _, ch := range channels {
		if ch.Status == channelStatusEnabled && contains(ch.Models, model) && (group == "" || contains(ch.Groups, group)) {
			candidates = append(candidates, ch)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ResponseTime < candidates[j].ResponseTime
	})
	return candidates
}

func channelTestEndpoint(channelID int, model string, stream bool) string {
	ep := fmt.Sprintf("/api/channel/test/%d?model=%s", channelID, url.QueryEscape(model))
	if stream {
		ep += "&stream=true"
	}
	return ep
}
