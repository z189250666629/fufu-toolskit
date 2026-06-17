package main

func applyManualToCell(c *ModelCell, rec testRecord, next int64) {
	c.ManualTest = rec
	c.NextTestAllowedAt = next
	if rec.OK {
		c.SuccessCount++
		c.LastSuccessAt = rec.TestedAt
	} else {
		c.FailureCount++
		c.LastFailureAt = rec.TestedAt
	}
	c.RequestCount = c.SuccessCount + c.FailureCount
	c.SuccessRate = rate(c.SuccessCount, c.FailureCount)
	c.Status = statusFromCounts(c.SuccessCount, c.FailureCount)
	c.LastSeenAt = maxInt64(c.LastSuccessAt, c.LastFailureAt)
}

func applyManual(ms *ModelStatus, siteName, model, group string, rec testRecord, next int64) {
	for i := range ms.Models {
		if ms.Models[i].Model == model {
			if c := ms.Models[i].PerSite[siteName]; c != nil {
				oldStatus := ms.Models[i].Status
				if group != "" {
					if groupCell := c.GroupStats[group]; groupCell != nil {
						applyManualToCell(groupCell, rec, next)
					}
				} else {
					applyManualToCell(c, rec, next)
				}
				recomputeModelRowSummary(&ms.Models[i])
				updateModelStatusTotalsForRowStatus(ms, oldStatus, ms.Models[i].Status)
			}
		}
	}
}

func applyManualToCachedStatus(siteName, model, group string, rec testRecord, next int64) {
	modelCache.Lock()
	defer modelCache.Unlock()
	if modelCache.Value == nil {
		return
	}
	nextStatus := cloneModelStatus(modelCache.Value)
	applyManual(nextStatus, siteName, model, group, rec, next)
	modelCache.Value = nextStatus
}

func cloneModelStatus(status *ModelStatus) *ModelStatus {
	if status == nil {
		return nil
	}
	clone := *status
	if status.Sites != nil {
		clone.Sites = make([]SiteStatus, len(status.Sites))
		for i, site := range status.Sites {
			clone.Sites[i] = site
			clone.Sites[i].Groups = append([]string(nil), site.Groups...)
		}
	}
	if status.Models != nil {
		clone.Models = make([]ModelRow, len(status.Models))
		for i := range status.Models {
			clone.Models[i] = cloneModelRow(status.Models[i])
		}
	}
	if status.Totals != nil {
		clone.Totals = make(map[string]int, len(status.Totals))
		for key, value := range status.Totals {
			clone.Totals[key] = value
		}
	}
	return &clone
}

func cloneModelRow(row ModelRow) ModelRow {
	clone := row
	if row.PerSite != nil {
		clone.PerSite = make(map[string]*ModelCell, len(row.PerSite))
		for key, cell := range row.PerSite {
			clone.PerSite[key] = cloneModelCell(cell)
		}
	}
	return clone
}

func cloneModelCell(cell *ModelCell) *ModelCell {
	if cell == nil {
		return nil
	}
	clone := *cell
	clone.Groups = append([]string(nil), cell.Groups...)
	if cell.Pricing != nil {
		price := *cell.Pricing
		clone.Pricing = &price
	}
	if cell.GroupStats != nil {
		clone.GroupStats = make(map[string]*ModelCell, len(cell.GroupStats))
		for key, groupCell := range cell.GroupStats {
			clone.GroupStats[key] = cloneModelCell(groupCell)
		}
	}
	return &clone
}
