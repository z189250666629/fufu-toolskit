package rawconv

import (
	"encoding/json"
	"math"
	"testing"
)

func TestInt64NormalizesRawNumbers(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
		want int64
	}{
		{name: "json decimal", in: json.Number("42.9"), want: 42},
		{name: "string decimal", in: "8.9", want: 8},
		{name: "bad string", in: "not-a-number", want: 0},
		{name: "nil", in: nil, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Int64(tc.in); got != tc.want {
				t.Fatalf("Int64(%#v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestInt64RejectsNonFiniteAndOverflow(t *testing.T) {
	for _, value := range []any{
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
		json.Number("9223372036854775808"),
		"9223372036854775808",
	} {
		t.Run("", func(t *testing.T) {
			if got := Int64(value); got != 0 {
				t.Fatalf("Int64(%#v) = %d, want 0", value, got)
			}
		})
	}
}

func TestFloat64NormalizesRawNumbers(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
		want float64
	}{
		{name: "json number", in: json.Number("1.25"), want: 1.25},
		{name: "string", in: "2.5", want: 2.5},
		{name: "int", in: 3, want: 3},
		{name: "bad string", in: "bad", want: 0},
		{name: "non finite", in: math.Inf(1), want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Float64(tc.in); got != tc.want {
				t.Fatalf("Float64(%#v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
