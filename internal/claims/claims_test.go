package claims

import (
	"strings"
	"testing"
)

func line(s string) string { return s + "\n" }

func assistantText(text string) string {
	return line(`{"type":"assistant","message":{"content":[{"type":"text","text":"` + text + `"}]}}`)
}

func bashRan(cmd, id string) string {
	use := line(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"` + id + `","name":"Bash","input":{"command":"` + cmd + `"}}]}}`)
	res := line(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"` + id + `","is_error":false}]}}`)
	return use + res
}

func bashUseNoResult(cmd, id string) string {
	return line(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"` + id + `","name":"Bash","input":{"command":"` + cmd + `"}}]}}`)
}

func reviewerRan(id string) string {
	use := line(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"` + id + `","name":"Agent","input":{"subagent_type":"sdd-reviewer","prompt":"review"}}]}}`)
	res := line(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"` + id + `","is_error":false}]}}`)
	return use + res
}

func TestSafetyClaimWithoutReviewerBlocks(t *testing.T) {
	transcript := assistantText("The endpoint is now secure and ready.")
	res := Analyze(strings.NewReader(transcript))
	if !res.Block {
		t.Fatal("safety claim without reviewer must block")
	}
	if !strings.Contains(res.Reason, "sdd-reviewer") {
		t.Errorf("reason should mention the reviewer, got %q", res.Reason)
	}
}

func TestSafetyClaimWithReviewerPasses(t *testing.T) {
	transcript := reviewerRan("r1") + assistantText("Reviewed; the endpoint is now secure.")
	res := Analyze(strings.NewReader(transcript))
	if res.Block {
		t.Errorf("reviewer ran, should not block: %q", res.Reason)
	}
	if !res.Evidence.ReviewerRan {
		t.Error("reviewer evidence not detected")
	}
}

func TestNegatedSafetyDoesNotBlock(t *testing.T) {
	for _, msg := range []string{
		"The endpoint is not secure yet.",
		"Implemented; it is not hardened.",
		"Is this safe to use in production?",
	} {
		res := Analyze(strings.NewReader(assistantText(msg)))
		if res.Block {
			t.Errorf("negated/interrogative safety must not block: %q -> %q", msg, res.Reason)
		}
	}
}

func TestAffirmativeSafetyWithoutReviewerBlocks(t *testing.T) {
	res := Analyze(strings.NewReader(assistantText("The login flow is now secure.")))
	if !res.Block {
		t.Error("affirmative safety claim without reviewer must block")
	}
}

func TestHedgedThenAssertedSafetyBlocks(t *testing.T) {
	// A negated claim must not shield a later affirmative one.
	res := Analyze(strings.NewReader(assistantText("It is not fully secure, but the auth layer is now secure.")))
	if !res.Block {
		t.Error("hedge-then-assert must still block on the affirmative claim")
	}
}

func TestNegationDoesNotBleedAcrossSentences(t *testing.T) {
	res := Analyze(strings.NewReader(assistantText("We did not ship docs. The auth is now secure.")))
	if !res.Block {
		t.Error("a 'not' in an unrelated earlier sentence must not suppress a real claim")
	}
}

func TestMentioningTestCommandIsNotEvidence(t *testing.T) {
	transcript := bashUseNoResult("echo go test", "b9") + assistantText("All tests pass.")
	res := Analyze(strings.NewReader(transcript))
	if res.Evidence.TestsRan {
		t.Error("a command merely mentioning 'go test' (no tool_result) is not evidence")
	}
	if !res.Block {
		t.Error("test claim without a real run must still block")
	}
}

func TestTestClaimWithoutRunBlocks(t *testing.T) {
	transcript := assistantText("All tests pass, we are good.")
	res := Analyze(strings.NewReader(transcript))
	if !res.Block {
		t.Fatal("test claim without a test run must block")
	}
}

func TestTestClaimWithRunPasses(t *testing.T) {
	transcript := bashRan("go test ./...", "b1") + assistantText("All tests pass.")
	res := Analyze(strings.NewReader(transcript))
	if res.Block {
		t.Errorf("tests ran, should not block: %q", res.Reason)
	}
}

func TestPortugueseClaims(t *testing.T) {
	res := Analyze(strings.NewReader(assistantText("Pronto, testes passando e tudo certo.")))
	if !res.Block {
		t.Fatal("Portuguese test claim without run must block")
	}
}

func TestOnlyLastAssistantMessageCounts(t *testing.T) {
	transcript := assistantText("This is now secure.") + assistantText("Implemented; security not verified.")
	res := Analyze(strings.NewReader(transcript))
	if res.Block {
		t.Errorf("earlier claims superseded by honest final message, should not block: %q", res.Reason)
	}
}

func TestReviewerToolUseWithoutResultIsNotEvidence(t *testing.T) {
	transcript := bashUseNoResult("noop", "x") +
		line(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"r2","name":"Agent","input":{"subagent_type":"sdd-reviewer"}}]}}`) +
		assistantText("It is now secure.")
	res := Analyze(strings.NewReader(transcript))
	if res.Evidence.ReviewerRan {
		t.Error("reviewer dispatched but no tool_result — not completed evidence")
	}
	if !res.Block {
		t.Error("safety claim with an incomplete review must block")
	}
}

func TestNeutralMessageDoesNotBlock(t *testing.T) {
	res := Analyze(strings.NewReader(assistantText("Refactored the parser and updated the docs.")))
	if res.Block {
		t.Errorf("neutral message must not block: %q", res.Reason)
	}
}

func TestGarbageLinesAreIgnored(t *testing.T) {
	transcript := line("not json at all{{{") + assistantText("All done.")
	res := Analyze(strings.NewReader(transcript))
	if res.Block {
		t.Error("garbage lines must not cause blocks")
	}
}

func TestReceiptEvidenceDetected(t *testing.T) {
	transcript := bashUseNoResult("claudiao receipt create --reviewer sdd-reviewer", "rc")
	res := Analyze(strings.NewReader(transcript))
	if !res.Evidence.ReceiptCreated {
		t.Error("receipt creation not detected")
	}
}
