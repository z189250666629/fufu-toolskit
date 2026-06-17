package combine

import "fufu/combinecore"

func validateExecuteMergeRequest(p ExecuteMergeParams, tokens []ResolvedToken) error {
	return combinecore.ValidateExecuteMergeRequest(coreExecuteMergeParams(p), coreResolvedTokens(tokens))
}

func isAllowedMergeIntervalUnit(unit int) bool {
	return combinecore.IsAllowedMergeIntervalUnit(unit)
}
