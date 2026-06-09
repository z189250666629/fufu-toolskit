package tokens

import (
	"strings"
	"testing"
)

func TestSplitSearchResultsPreservesFoundAndMissingOrder(t *testing.T) {
	foundToken := Token{ID: 7, Key: "sk-found"}
	found, missing := splitSearchResults([]SearchResult{
		{Key: "sk-missing-a"},
		{Key: "sk-found", Found: &foundToken},
		{Key: "sk-missing-b"},
	})

	if len(found) != 1 || found[0].ID != 7 {
		t.Fatalf("found = %#v", found)
	}
	if strings.Join(missing, ",") != "sk-missing-a,sk-missing-b" {
		t.Fatalf("missing = %#v", missing)
	}
}
