package main

import "fufu/modelcore"

type modelStatusChannelIndex struct {
	groups          []string
	channelsByModel map[string][]Channel
}

func indexChannelsForModelStatus(channels []Channel) modelStatusChannelIndex {
	idx := modelcore.IndexChannelsForModelStatus(channels)
	return modelStatusChannelIndex{groups: idx.Groups, channelsByModel: idx.ChannelsByModel}
}
