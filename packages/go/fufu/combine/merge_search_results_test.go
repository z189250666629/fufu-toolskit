package combine

import (
	"strings"
	"testing"
)

func TestSplitSearchTokenResultsPreservesFoundAndMissingOrder(t *testing.T) {
	foundToken := ResolvedToken{ID: 7, Key: "sk-found"}
	found, missing := splitSearchTokenResults([]SearchTokenResult{
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

func TestMissingTokenErrorMasksKeys(t *testing.T) {
	err := missingTokenError([]string{"sk-abcdefghijkl"})
	if err == nil || !strings.Contains(err.Error(), "sk-abcd…ijkl") {
		t.Fatalf("missing token err = %v", err)
	}
	if err := missingTokenError(nil); err != nil {
		t.Fatalf("empty missing token err = %v", err)
	}
}
