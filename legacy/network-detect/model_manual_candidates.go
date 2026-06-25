package main

import (
	"fmt"
	"fufu/modelcore"
	"net/url"
)

func selectModelTestChannels(channels []Channel, model, group string) []Channel {
	return modelcore.SelectModelTestChannels(channels, model, group)
}

func channelResponseTimeRank(responseTime int64) int64 {
	return modelcore.ChannelResponseTimeRank(responseTime)
}

func channelTestEndpoint(channelID int, model string, stream bool) string {
	ep := fmt.Sprintf("/api/channel/test/%d?model=%s", channelID, url.QueryEscape(model))
	if stream {
		ep += "&stream=true"
	}
	return ep
}
