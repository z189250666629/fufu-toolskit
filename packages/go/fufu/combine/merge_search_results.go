package combine

import (
	"fmt"
	"strings"
)

func splitSearchTokenResults(results []SearchTokenResult) ([]ResolvedToken, []string) {
	found := []ResolvedToken{}
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

func missingTokenError(missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	shown := []string{}
	for _, k := range missing {
		shown = append(shown, displayKey(k))
	}
	return fmt.Errorf("未找到令牌: %s", strings.Join(shown, ", "))
}
