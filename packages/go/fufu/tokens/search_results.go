package tokens

func splitSearchResults(results []SearchResult) ([]Token, []string) {
	found := []Token{}
	missing := []string{}
	for _, r := range results {
		if r.Found != nil {
			found = append(found, *r.Found)
		} else {
			missing = append(missing, r.Key)
		}
	}
	return found, missing
}
