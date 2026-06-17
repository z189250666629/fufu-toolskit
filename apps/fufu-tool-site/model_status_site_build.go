package main

type builtModelStatusSite struct {
	Status              SiteStatus
	ChannelIndex        modelStatusChannelIndex
	SuccessByModel      map[string][]LogRow
	ErrorByModel        map[string][]LogRow
	SuccessByModelGroup map[string][]LogRow
	ErrorByModelGroup   map[string][]LogRow
	Pricing             map[string]Pricing
}

func buildPerSiteModelStatus(data modelStatusSiteData) builtModelStatusSite {
	channelIndex := indexChannelsForModelStatus(data.Channels)
	successLogs := data.SuccessLogs
	errorLogs := data.ErrorLogs
	siteStatus := SiteStatus{
		Site:          publicModelSite(data.Site),
		Groups:        channelIndex.groups,
		LogError:      data.LogError,
		ChannelsError: data.ChannelsError,
		PricingError:  data.PricingError,
	}
	siteStatus.SuccessCount = len(successLogs)
	siteStatus.FailureCount = len(errorLogs)
	siteStatus.RequestCount = siteStatus.SuccessCount + siteStatus.FailureCount
	siteStatus.SuccessRate = rate(siteStatus.SuccessCount, siteStatus.FailureCount)
	siteStatus.LastSeenAt = maxLogTime(append(successLogs, errorLogs...))
	siteStatus.Status = siteStatusFromCounts(siteStatus.SuccessCount, siteStatus.FailureCount)

	return builtModelStatusSite{
		Status:              siteStatus,
		ChannelIndex:        channelIndex,
		SuccessByModel:      groupLogs(successLogs),
		ErrorByModel:        groupLogs(errorLogs),
		SuccessByModelGroup: groupLogsByModelGroup(successLogs),
		ErrorByModelGroup:   groupLogsByModelGroup(errorLogs),
		Pricing:             data.Pricing,
	}
}

func buildCell(siteName, model string, chans []Channel, success, errorRows []LogRow, pricing Pricing) *ModelCell {
	enabled := 0
	groups := map[string]bool{}
	for _, ch := range chans {
		if ch.Status == channelStatusEnabled {
			enabled++
		}
		for _, g := range ch.Groups {
			groups[g] = true
		}
	}
	cell := &ModelCell{Configured: true, SiteName: siteName, Model: model, RequestCount: len(success) + len(errorRows), SuccessCount: len(success), FailureCount: len(errorRows), SuccessRate: rate(len(success), len(errorRows)), LastSuccessAt: maxLogTime(success), LastFailureAt: maxLogTime(errorRows), TotalChannelCount: len(chans), EnabledChannelCount: enabled, Groups: keys(groups)}
	cell.LastSeenAt = maxInt64(cell.LastSuccessAt, cell.LastFailureAt)
	cell.Status = statusFromCounts(cell.SuccessCount, cell.FailureCount)
	if pricing.Input != 0 || pricing.Output != 0 || pricing.Request != 0 || pricing.Type != "" {
		p := pricing
		cell.Pricing = &p
	}
	return cell
}
