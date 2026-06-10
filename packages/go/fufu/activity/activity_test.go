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

func TestDefaultConfigReturnsIndependentCopies(t *testing.T) {
	spinMap := DefaultSpinMap()
	spinMap[100] = 999
	if DefaultSpinMap()[100] == 999 {
		t.Fatalf("DefaultSpinMap should return an independent map copy")
	}

	prizePool := DefaultPrizePool()
	prizePool[2].Weight = 1
	if DefaultPrizePool()[2].Weight == 1 {
		t.Fatalf("DefaultPrizePool should return an independent slice copy")
	}

	tierPools := DefaultTierPools()
	tierPools[100][2].Weight = 1
	if DefaultTierPools()[100][2].Weight == 1 {
		t.Fatalf("DefaultTierPools should return independent nested slice copies")
	}

	postJackpotPool := DefaultPostJackpotPool()
	postJackpotPool[2].Weight = 1
	if DefaultPostJackpotPool()[2].Weight == 1 {
		t.Fatalf("DefaultPostJackpotPool should return an independent slice copy")
	}

	scratchRewards := DefaultScratchRewards()
	scratchRewards[0] = 999
	if DefaultScratchRewards()[0] == 999 {
		t.Fatalf("DefaultScratchRewards should return an independent slice copy")
	}
}
