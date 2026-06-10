package combine

import "testing"

func TestUniqueTraceKeyHashesNormalizesAndDedupes(t *testing.T) {
	hashes, set := uniqueTraceKeyHashes([]string{" abcdefghijkl ", "sk-abcdefghijkl", "", "defghijklmno"})

	if len(hashes) != 2 || hashes[0] != keyHash("sk-abcdefghijkl") || hashes[1] != keyHash("sk-defghijklmno") {
		t.Fatalf("hashes = %#v", hashes)
	}
	if !set[keyHash("sk-abcdefghijkl")] || !set[keyHash("sk-defghijklmno")] || len(set) != 2 {
		t.Fatalf("set = %#v", set)
	}
}

func TestTraceDirectionFromMatches(t *testing.T) {
	cases := []struct {
		source bool
		result bool
		want   string
	}{
		{source: true, result: true, want: "both"},
		{result: true, want: "result"},
		{source: true, want: "source"},
		{want: "related"},
	}

	for _, tc := range cases {
		if got := traceDirectionFromMatches(tc.source, tc.result); got != tc.want {
			t.Fatalf("traceDirectionFromMatches(%v,%v)=%q want %q", tc.source, tc.result, got, tc.want)
		}
	}
}
func TestAppendNewTraceMergeIDsSkipsSeenAndPreservesOrder(t *testing.T) {
	seen := map[int64]bool{2: true}
	all, fresh := appendNewTraceMergeIDs([]int64{1, 2, 3, 1}, seen, []int64{9})

	if len(all) != 3 || all[0] != 9 || all[1] != 1 || all[2] != 3 {
		t.Fatalf("all merge IDs = %#v", all)
	}
	if len(fresh) != 2 || fresh[0] != 1 || fresh[1] != 3 {
		t.Fatalf("fresh merge IDs = %#v", fresh)
	}
	if !seen[1] || !seen[2] || !seen[3] || len(seen) != 3 {
		t.Fatalf("seen = %#v", seen)
	}
}

func TestNextUnseenTraceHashesSkipsBlankAndSeen(t *testing.T) {
	seen := map[string]bool{"a": true}
	next := nextUnseenTraceHashes([]string{"", "a", "b", " b ", "c"}, seen)

	if len(next) != 3 || next[0] != "b" || next[1] != " b " || next[2] != "c" {
		t.Fatalf("next = %#v", next)
	}
	if !seen["a"] || !seen["b"] || !seen[" b "] || !seen["c"] || len(seen) != 4 {
		t.Fatalf("seen = %#v", seen)
	}
}
