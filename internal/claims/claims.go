// Package claims is the mechanized version of the agreement's hardest rule:
// never declare "secure" or "tests passing" without evidence. The Stop hook
// scans the final assistant message for such claims and the transcript for
// the evidence that would back them; a claim without evidence blocks the
// stop with instructions to either produce the evidence or retract.
package claims

import (
	"bufio"
	"encoding/json"
	"io"
	"regexp"
	"strings"
)

type Evidence struct {
	ReviewerRan    bool
	TestsRan       bool
	ReceiptCreated bool
}

type Result struct {
	Block    bool
	Reason   string
	Claims   []string
	Evidence Evidence
}

var (
	// Affirmative safety assertions only — "is secure", "now hardened".
	// Negations and questions ("not secure", "is this safe?") must not match.
	safetyClaim = regexp.MustCompile(`(?i)\b(is|are|now|fully|está|estão|é|ficou)\s+(\w+\s+){0,2}(secure|hardened|protegid[oa]|invulnerável|safe (to|for) (use|production))\b`)
	negatedNear = regexp.MustCompile(`(?i)\b(not|never|n[ãa]o|isn'?t|aren'?t|without being|t[oa]\??$)\b`)

	testClaim = regexp.MustCompile(`(?i)(all tests pass\w*|tests? (are |estão )?(passing|green)|testes? (estão )?(passando|verdes?)|testes? passaram|suite (is )?(green|verde)|100% (of |dos )?test)`)

	testCommand = regexp.MustCompile(`(?i)\b(go test|gotestsum|npm (run )?test|pnpm (run )?test|yarn test|pytest|python -m pytest|cargo test|vitest|jest|make test|mvn test|gradle test|bun test)\b`)
)

type transcriptLine struct {
	Type    string `json:"type"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Name      string          `json:"name"`
	ID        string          `json:"id"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Input     json.RawMessage `json:"input"`
}

// Analyze reads a Claude Code transcript (JSONL) and decides whether the
// final assistant message makes claims the transcript cannot back. Evidence
// counts only when a tool_use has a matching tool_result — the model writing
// "go test" in a command string is not proof the suite ran.
func Analyze(r io.Reader) Result {
	pendingTest := map[string]bool{}
	pendingReviewer := map[string]bool{}
	var ev Evidence
	var lastText string

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 256*1024), 16*1024*1024)
	for sc.Scan() {
		var line transcriptLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue // transcript is untrusted input: skip what we cannot parse
		}
		var blocks []contentBlock
		if err := json.Unmarshal(line.Message.Content, &blocks); err != nil {
			continue
		}

		if line.Type == "assistant" {
			var texts []string
			for _, b := range blocks {
				switch b.Type {
				case "text":
					texts = append(texts, b.Text)
				case "tool_use":
					registerToolUse(b, pendingTest, pendingReviewer, &ev)
				}
			}
			if len(texts) > 0 {
				lastText = strings.Join(texts, "\n")
			}
			continue
		}

		// user / tool messages carry the tool_result that proves execution.
		for _, b := range blocks {
			if b.Type != "tool_result" || b.ToolUseID == "" {
				continue
			}
			if pendingTest[b.ToolUseID] && !b.IsError {
				ev.TestsRan = true
			}
			if pendingReviewer[b.ToolUseID] && !b.IsError {
				ev.ReviewerRan = true
			}
		}
	}

	res := Result{Evidence: ev}
	if lastText == "" {
		return res
	}

	if m := findAffirmativeSafety(lastText); m != "" && !ev.ReviewerRan {
		res.Claims = append(res.Claims, "safety: "+m)
		res.Block = true
		res.Reason = "Your final message claims safety (\"" + m + "\") but the adversarial reviewer (sdd-reviewer) never ran to completion in this session. " +
			"Either run the reviewer now and report its findings, or rephrase to \"implemented; security not verified\"."
	}
	if m := testClaim.FindString(lastText); m != "" && !ev.TestsRan {
		res.Claims = append(res.Claims, "tests: "+m)
		res.Block = true
		if res.Reason != "" {
			res.Reason += " Also: "
		}
		res.Reason += "Your final message claims tests pass (\"" + m + "\") but no test command produced a result in this session. " +
			"Run the test suite and show its output, or remove the claim."
	}
	return res
}

// findAffirmativeSafety returns the first safety claim that is actually
// asserted — not negated, not a question. It inspects every match (a message
// can hedge then assert: "not fully secure, but the auth is now secure") and
// bounds the negation lookbehind to the current sentence so a "not" in an
// unrelated earlier sentence cannot suppress a real claim.
func findAffirmativeSafety(text string) string {
	for _, loc := range safetyClaim.FindAllStringIndex(text, -1) {
		start := sentenceStart(text, loc[0])
		if neg := loc[0] - 15; neg > start {
			start = neg // negation must be close, not just same-sentence
		}
		if negatedNear.MatchString(text[start:loc[1]]) {
			continue
		}
		if endsSentenceWithQuestion(text, loc[1]) {
			continue
		}
		return text[loc[0]:loc[1]]
	}
	return ""
}

func sentenceStart(text string, pos int) int {
	for i := pos - 1; i >= 0; i-- {
		switch text[i] {
		case '.', '!', '?', '\n':
			return i + 1
		}
	}
	return 0
}

func endsSentenceWithQuestion(text string, from int) bool {
	for i := from; i < len(text); i++ {
		switch text[i] {
		case '?':
			return true
		case '.', '!', '\n':
			return false
		}
	}
	return false
}

func registerToolUse(b contentBlock, pendingTest, pendingReviewer map[string]bool, ev *Evidence) {
	switch b.Name {
	case "Task", "Agent":
		var in struct {
			SubagentType string `json:"subagent_type"`
		}
		if b.ID != "" && json.Unmarshal(b.Input, &in) == nil && strings.Contains(in.SubagentType, "sdd-reviewer") {
			pendingReviewer[b.ID] = true
		}
	case "Bash":
		var in struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(b.Input, &in) != nil {
			return
		}
		if b.ID != "" && testCommand.MatchString(in.Command) {
			pendingTest[b.ID] = true
		}
		if strings.Contains(in.Command, "claudiao receipt create") {
			ev.ReceiptCreated = true
		}
	}
}
