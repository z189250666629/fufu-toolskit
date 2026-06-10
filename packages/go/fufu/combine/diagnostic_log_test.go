package combine

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSearchKeysRedactsLookupErrorsFromLogs(t *testing.T) {
	var logs bytes.Buffer
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	})

	app := NewApp(Config{URL: "http://internal.example.local/%zz", Token: "secret-token", UserID: "1"}, nil)
	secretKey := "sk-public-search-secret-1234567890"
	req := httptest.NewRequest(http.MethodPost, "/api/search-keys", strings.NewReader(`{"keys":["`+secretKey+`"]}`))
	w := httptest.NewRecorder()

	app.handleSearchKeys(w, req)

	text := logs.String()
	if !strings.Contains(text, "combine search token lookup failed") {
		t.Fatalf("expected lookup failure to be logged, got %q", text)
	}
	for _, leaked := range []string{secretKey, "public-search-secret", "1234567890", "token=", "internal.example.local", "%zz", "secret-token"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("lookup error log leaked %q in %q", leaked, text)
		}
	}
}
