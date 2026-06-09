package combine

import (
	"errors"
	"testing"
)

func TestBuildMergeDeleteOutcomeSuccess(t *testing.T) {
	out := buildMergeDeleteOutcome(ResolvedToken{ID: 7, Key: "sk-source"}, true, APIResponse{}, nil)

	if !out.result.OK || out.result.ID != 7 || out.result.Key != "sk-source" {
		t.Fatalf("result = %#v", out.result)
	}
	if out.failureKey != "" || out.traceMessage != "" {
		t.Fatalf("unexpected failure fields = %#v", out)
	}
}

func TestBuildMergeDeleteOutcomeUsesErrorMessage(t *testing.T) {
	out := buildMergeDeleteOutcome(ResolvedToken{ID: 7, Key: "sk-source"}, false, APIResponse{StatusCode: 502}, errors.New("network down"))

	if out.result.OK || out.failureKey != "sk-source" || out.traceMessage != "network down" {
		t.Fatalf("error outcome = %#v", out)
	}
}

func TestBuildMergeDeleteOutcomeUsesUpstreamStatus(t *testing.T) {
	out := buildMergeDeleteOutcome(ResolvedToken{ID: 7, Key: "sk-source"}, false, APIResponse{StatusCode: 502}, nil)

	if out.result.OK || out.failureKey != "sk-source" || out.traceMessage == "" {
		t.Fatalf("upstream outcome = %#v", out)
	}
}
