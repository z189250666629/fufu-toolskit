package combine

import "sort"

func sortTraceResultsByCreatedAt(results []TraceResult) {
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].CreatedAt < results[j].CreatedAt
	})
}
