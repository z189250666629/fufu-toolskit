package combine

import (
	"net/url"
	"regexp"
	"strings"
)

const maxTraceDiagnosticRunes = 512

var (
	traceSecretKeyPattern     = regexp.MustCompile(`sk-[A-Za-z0-9][A-Za-z0-9_-]{6,}`)
	traceBearerPattern        = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	traceAuthorizationPattern = regexp.MustCompile(`(?i)\bAuthorization\s*:\s*[^\s]+(?:\s+[^\s]+)?`)
	traceTokenParamPattern    = regexp.MustCompile(`(?i)\b(token=)[^&\s;]+`)
	traceURLPattern           = regexp.MustCompile(`https?://[^\s"'<>]+`)
)

func redactTraceDiagnostic(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	message = traceSecretKeyPattern.ReplaceAllStringFunc(message, keyMask)
	message = traceBearerPattern.ReplaceAllString(message, "Bearer [REDACTED]")
	message = traceAuthorizationPattern.ReplaceAllString(message, "Authorization: [REDACTED]")
	message = traceTokenParamPattern.ReplaceAllString(message, "${1}[REDACTED]")
	message = traceURLPattern.ReplaceAllStringFunc(message, redactTraceDiagnosticURL)
	return truncateTraceDiagnostic(message)
}

func redactTraceDiagnosticURL(raw string) string {
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

func truncateTraceDiagnostic(message string) string {
	runes := []rune(message)
	if len(runes) <= maxTraceDiagnosticRunes {
		return message
	}
	return string(runes[:maxTraceDiagnosticRunes]) + "…"
}
