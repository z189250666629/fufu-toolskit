package combine

import "fufu/combinecore"

func evaluatePublicMergeEligibility(tokens []ResolvedToken) PublicMergeEligibility {
	elig := combinecore.EvaluatePublicMergeEligibility(coreResolvedTokens(tokens))
	return PublicMergeEligibility{Eligible: elig.Eligible, Reasons: elig.Reasons}
}
