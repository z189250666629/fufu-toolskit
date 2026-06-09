package combine

import "testing"

func TestAppendTraceTokenClassifiesSourceAndResultMatches(t *testing.T) {
	trace := TraceResult{}
	queryHashes := map[string]bool{"source-hash": true}

	sourceMatched, resultMatched := appendTraceToken(&trace, "source", TraceToken{KeyHash: "source-hash", Key: "sk-source"}, queryHashes)
	if !sourceMatched || resultMatched {
		t.Fatalf("source match = %v/%v", sourceMatched, resultMatched)
	}
	if len(trace.SourceKeys) != 1 || trace.SourceKeys[0].Key != "sk-source" {
		t.Fatalf("source keys = %#v", trace.SourceKeys)
	}

	sourceMatched, resultMatched = appendTraceToken(&trace, "result", TraceToken{KeyHash: "result-hash", Key: "sk-result"}, queryHashes)
	if sourceMatched || resultMatched {
		t.Fatalf("unexpected result match = %v/%v", sourceMatched, resultMatched)
	}
	if trace.ResultKey == nil || trace.ResultKey.Key != "sk-result" {
		t.Fatalf("result key = %#v", trace.ResultKey)
	}
}

func TestAppendTraceTokenIgnoresUnknownKind(t *testing.T) {
	trace := TraceResult{}
	sourceMatched, resultMatched := appendTraceToken(&trace, "other", TraceToken{KeyHash: "source-hash"}, map[string]bool{"source-hash": true})

	if sourceMatched || resultMatched || len(trace.SourceKeys) != 0 || trace.ResultKey != nil {
		t.Fatalf("unexpected trace mutation: %#v match=%v/%v", trace, sourceMatched, resultMatched)
	}
}
