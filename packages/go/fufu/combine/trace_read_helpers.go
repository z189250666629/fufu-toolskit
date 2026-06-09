package combine

import "strings"

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

func appendNewTraceMergeIDs(ids []int64, seen map[int64]bool, mergeIDs []int64) ([]int64, []int64) {
	newIDs := []int64{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		mergeIDs = append(mergeIDs, id)
		newIDs = append(newIDs, id)
	}
	return mergeIDs, newIDs
}

func nextUnseenTraceHashes(relatedHashes []string, seen map[string]bool) []string {
	next := []string{}
	for _, hash := range relatedHashes {
		if strings.TrimSpace(hash) == "" || seen[hash] {
			continue
		}
		seen[hash] = true
		next = append(next, hash)
	}
	return next
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
