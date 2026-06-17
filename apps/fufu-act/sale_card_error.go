package activityapp

import (
	"net/url"
	"regexp"
	"strings"
)

const maxSaleCardDiagnosticRunes = 512

var (
	saleCardSecretKeyPattern     = regexp.MustCompile(`sk-[A-Za-z0-9][A-Za-z0-9_-]{6,}`)
	saleCardBearerPattern        = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	saleCardAuthorizationPattern = regexp.MustCompile(`(?i)\bAuthorization\s*:\s*[^\s]+(?:\s+[^\s]+)?`)
	saleCardQuerySecretPattern   = regexp.MustCompile(`(?i)\b(token|password|secret|api[_-]?key)=([^&\s;]+)`)
	saleCardJSONSecretPattern    = regexp.MustCompile(`(?i)("?(?:token|password|secret|authorization|api[_-]?key)"?\s*:\s*")([^"]+)(")`)
	saleCardURLPattern           = regexp.MustCompile(`https?://[^\s"'<>]+`)
)

func saleCardGenerationFailureMessage(err error) string {
	detail := saleCardGenerationFailureDetail(err)
	if detail == "" {
		return "次数 fufu 生成卡密失败"
	}
	return "次数 fufu 生成卡密失败：" + detail
}

func saleCardGenerationFailureDetail(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	sentinel := ErrSaleCardGenerationFailed.Error()
	if idx := strings.Index(message, sentinel); idx >= 0 {
		message = message[idx+len(sentinel):]
	}
	message = strings.TrimLeft(strings.TrimSpace(message), ":;- ")
	if message == "" || message == sentinel {
		return ""
	}
	return redactSaleCardDiagnostic(message)
}

func redactSaleCardDiagnostic(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	message = saleCardSecretKeyPattern.ReplaceAllString(message, "sk-[REDACTED]")
	message = saleCardBearerPattern.ReplaceAllString(message, "Bearer [REDACTED]")
	message = saleCardAuthorizationPattern.ReplaceAllString(message, "Authorization: [REDACTED]")
	message = saleCardQuerySecretPattern.ReplaceAllString(message, "${1}=[REDACTED]")
	message = saleCardJSONSecretPattern.ReplaceAllString(message, "${1}[REDACTED]${3}")
	message = saleCardURLPattern.ReplaceAllStringFunc(message, redactSaleCardDiagnosticURL)
	return truncateSaleCardDiagnostic(message)
}

func redactSaleCardDiagnosticURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[URL已脱敏]"
	}
	path := strings.TrimSpace(parsed.EscapedPath())
	if path == "" {
		return "[URL已脱敏]"
	}
	return "[URL已脱敏 " + path + "]"
}

func truncateSaleCardDiagnostic(message string) string {
	runes := []rune(message)
	if len(runes) <= maxSaleCardDiagnosticRunes {
		return message
	}
	return string(runes[:maxSaleCardDiagnosticRunes]) + "..."
}
