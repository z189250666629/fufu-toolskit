package combine

import "fufu/combinecore"

func coreRole(role Role) combinecore.Role {
	return combinecore.Role(role)
}

func coreExecuteMergeParams(p ExecuteMergeParams) combinecore.ExecuteMergeParams {
	return combinecore.ExecuteMergeParams{
		Keys:         p.Keys,
		IntervalUnit: p.IntervalUnit,
		TotalQuota:   p.TotalQuota,
		Name:         p.Name,
		CustomQuota:  p.CustomQuota,
		Role:         coreRole(p.Role),
		JobID:        p.JobID,
	}
}

func coreResolvedToken(token ResolvedToken) combinecore.ResolvedToken {
	return combinecore.ResolvedToken{
		ID:           token.ID,
		Key:          token.Key,
		DisplayKey:   displayKey(token.Key),
		RemainQuota:  token.RemainQuota,
		UsedQuota:    token.UsedQuota,
		IntervalUnit: token.IntervalUnit,
		Group:        token.Group,
		Status:       token.Status,
	}
}

func coreResolvedTokens(tokens []ResolvedToken) []combinecore.ResolvedToken {
	out := make([]combinecore.ResolvedToken, 0, len(tokens))
	for _, token := range tokens {
		out = append(out, coreResolvedToken(token))
	}
	return out
}
