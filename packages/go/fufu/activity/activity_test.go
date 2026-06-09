package activity

import "testing"

func TestSpinGuaranteeForThousandCard(t *testing.T) {
	got := Spin(1000, false, 9, 10, 0, 0, func(max int) int { return 0 })
	if got.Type != "win" || got.Dollars != 100 {
		t.Fatalf("unexpected guarantee result: %#v", got)
	}
}

func TestDollarsTier(t *testing.T) {
	if DollarsTier(1_500_000, 500_000) != 3 {
		t.Fatalf("bad tier")
	}
}
