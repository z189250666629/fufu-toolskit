package tokens

func truncateTokenName(name string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(name)
	if len(runes) <= maxRunes {
		return name
	}
	return string(runes[:maxRunes])
}
