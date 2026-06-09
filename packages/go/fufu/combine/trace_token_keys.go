package combine

type traceTokenKeyParts struct {
	full string
	hash string
	mask string
}

func traceTokenKeys(token ResolvedToken) traceTokenKeyParts {
	full := ensureFullKey(token.Key)
	return traceTokenKeyParts{
		full: full,
		hash: keyHash(full),
		mask: keyMask(full),
	}
}
