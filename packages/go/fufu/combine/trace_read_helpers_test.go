package combine

import "testing"

func TestUniqueTraceKeyHashesNormalizesAndDedupes(t *testing.T) {
	hashes, set := uniqueTraceKeyHashes([]string{" abc ", "sk-abc", "", "def"})

	if len(hashes) != 2 || hashes[0] != keyHash("sk-abc") || hashes[1] != keyHash("sk-def") {
		t.Fatalf("hashes = %#v", hashes)
	}
	if !set[keyHash("sk-abc")] || !set[keyHash("sk-def")] || len(set) != 2 {
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
