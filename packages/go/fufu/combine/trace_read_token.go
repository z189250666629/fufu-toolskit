package combine

func appendTraceToken(trace *TraceResult, kind string, token TraceToken, queryHashes map[string]bool) (matchedSource bool, matchedResult bool) {
	switch kind {
	case "source":
		trace.SourceKeys = append(trace.SourceKeys, token)
		return queryHashes[token.KeyHash], false
	case "result":
		trace.ResultKey = &token
		return false, queryHashes[token.KeyHash]
	default:
		return false, false
	}
}
