package main

type modelStatusChannelIndex struct {
	groups          []string
	channelsByModel map[string][]Channel
}

func indexChannelsForModelStatus(channels []Channel) modelStatusChannelIndex {
	groupSet := map[string]bool{}
	channelsByModel := map[string][]Channel{}
	for _, ch := range channels {
		for _, g := range ch.Groups {
			groupSet[g] = true
		}
		for _, m := range ch.Models {
			channelsByModel[m] = append(channelsByModel[m], ch)
		}
	}
	return modelStatusChannelIndex{
		groups:          keys(groupSet),
		channelsByModel: channelsByModel,
	}
}
