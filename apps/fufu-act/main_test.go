package main

import "testing"

func TestSpinGuaranteeForThousandCard(t *testing.T) {
	got := spin(1000, false, 9, 10, 0, 0)
	if got.Type != "win" || got.Dollars != 100 {
		t.Fatalf("unexpected guarantee result: %#v", got)
	}
}

func TestScratchJSONHelpers(t *testing.T) {
	arr := jsonArr(`[1,2]`)
	if len(arr) != 2 || !intContains(arr, 2) || intContains(arr, 3) {
		t.Fatalf("bad json helpers: %#v", arr)
	}
}
