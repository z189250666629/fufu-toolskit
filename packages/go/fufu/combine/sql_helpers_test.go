package combine

import "testing"

func TestSQLPlaceholders(t *testing.T) {
	cases := []struct {
		count int
		want  string
	}{
		{count: 0, want: ""},
		{count: 1, want: "?"},
		{count: 3, want: "?,?,?"},
	}
	for _, tc := range cases {
		if got := sqlPlaceholders(tc.count); got != tc.want {
			t.Fatalf("sqlPlaceholders(%d) = %q, want %q", tc.count, got, tc.want)
		}
	}
}
