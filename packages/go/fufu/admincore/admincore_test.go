package admincore

import (
	"fmt"
	"testing"
)

func TestNormalizeHTTPSOriginKeepsOnlySchemeAndHost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"example.test/path", "https://example.test"},
		{"http://example.test/admin", "https://example.test"},
		{"https://example.test/admin/?x=1", "https://example.test"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeHTTPSOrigin(c.in); got != c.want {
			t.Fatalf("NormalizeHTTPSOrigin(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeNamedURLsFoldsLegacyAndDedupes(t *testing.T) {
	got := NormalizeNamedURLs(
		[]NamedURL{{Name: "primary", URL: "https://a.example.test/"}, {URL: "https://b.example.test"}},
		"https://b.example.test",
		func(index int) string { return fmt.Sprintf("line%d", index+1) },
	)

	if len(got) != 2 {
		t.Fatalf("urls = %#v", got)
	}
	if got[0].Name != "primary" || got[0].URL != "https://a.example.test" {
		t.Fatalf("first url = %#v", got[0])
	}
	if got[1].Name != "line2" || got[1].URL != "https://b.example.test" {
		t.Fatalf("second url = %#v", got[1])
	}
}

func TestMergeNamedURLsDedupePreservingOrder(t *testing.T) {
	got := MergeNamedURLs(
		[]NamedURL{{URL: "https://a.example.test"}, {URL: "https://b.example.test"}},
		[]NamedURL{{URL: "https://b.example.test"}, {URL: "https://c.example.test"}},
	)

	if len(got) != 3 || got[0].URL != "https://a.example.test" || got[1].URL != "https://b.example.test" || got[2].URL != "https://c.example.test" {
		t.Fatalf("merged urls = %#v", got)
	}
}

func TestMaskSecret(t *testing.T) {
	if got := MaskSecret("abcd1234wxyz"); got != "abcd…wxyz" {
		t.Fatalf("MaskSecret long = %q", got)
	}
	if got := MaskSecret("short"); got != "••••" {
		t.Fatalf("MaskSecret short = %q", got)
	}
}
