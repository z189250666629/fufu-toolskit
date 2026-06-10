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
		left := channelResponseTimeRank(candidates[i].ResponseTime)
		right := channelResponseTimeRank(candidates[j].ResponseTime)
		if left != right {
			return left < right
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates
}

func channelResponseTimeRank(responseTime int64) int64 {
	if responseTime <= 0 {
		return 1<<62 - 1
	}
	return responseTime
}

func channelTestEndpoint(channelID int, model string, stream bool) string {
	ep := fmt.Sprintf("/api/channel/test/%d?model=%s", channelID, url.QueryEscape(model))
	if stream {
		ep += "&stream=true"
	}
	return ep
}
