package combine

func uniqueTraceKeyHashes(rawKeys []string) ([]string, map[string]bool) {
	keys := normalizeKeys(rawKeys)
	hashSet := map[string]bool{}
	hashes := []string{}
	for _, key := range keys {
		hash := keyHash(key)
		if !hashSet[hash] {
			hashSet[hash] = true
			hashes = append(hashes, hash)
		}
	}
	return hashes, hashSet
}

func traceDirectionFromMatches(matchedSource, matchedResult bool) string {
	switch {
	case matchedSource && matchedResult:
		return "both"
	case matchedResult:
		return "result"
	case matchedSource:
		return "source"
	default:
		return "related"
	}
}
