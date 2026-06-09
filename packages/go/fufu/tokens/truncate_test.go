package tokens

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateTokenNamePreservesUTF8(t *testing.T) {
	name := "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十😀尾"

	got := truncateTokenName(name, 30)
	if !utf8.ValidString(got) {
		t.Fatalf("truncated name is not valid UTF-8: %q", got)
	}
	if utf8.RuneCountInString(got) != 30 {
		t.Fatalf("rune count = %d, value=%q", utf8.RuneCountInString(got), got)
	}
}

func TestTruncateTokenNameKeepsShortName(t *testing.T) {
	if got := truncateTokenName("短名称", 30); got != "短名称" {
		t.Fatalf("short name = %q", got)
	}
}
