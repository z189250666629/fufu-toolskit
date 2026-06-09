package combine

import (
	"fmt"
	"strings"
)

func validateVerifiedSourceToken(requested *ResolvedToken, verified ResolvedToken) error {
	if requested == nil {
		return fmt.Errorf("Token %d 校验失败", verified.ID)
	}
	if strings.TrimPrefix(verified.Key, "sk-") != strings.TrimPrefix(requested.Key, "sk-") {
		return fmt.Errorf("%s 校验失败，请重试", displayKey(requested.Key))
	}
	if verified.Status != 1 {
		return fmt.Errorf("%s 已被禁用，无法参与合卡", displayKey(verified.Key))
	}
	return nil
}
