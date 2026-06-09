package combine

import "testing"

func TestSortTraceResultsByCreatedAtKeepsStableOrder(t *testing.T) {
	results := []TraceResult{
		{MergeID: 3, CreatedAt: 20},
		{MergeID: 1, CreatedAt: 10},
		{MergeID: 2, CreatedAt: 10},
	}

	sortTraceResultsByCreatedAt(results)

	got := []int64{results[0].MergeID, results[1].MergeID, results[2].MergeID}
	want := []int64{1, 2, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %#v", got)
		}
	}
}
