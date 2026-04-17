# Rules — Token economy & concision

## Principle

Responses go straight to the point. Every token spent on preamble
or courtesy is a token unavailable for useful context.

## Forbidden

- Preambles: "Sure!", "Of course!", "Great question!", "I'll help you with..."
- Narrating what you are about to do before doing it: "Let me analyze...",
  "I'll start by reading...", "Now I will edit..."
- Restating the user's request before answering
- Empty closings: "Hope that helps!", "Let me know if..."
- Summaries at the end of a turn when the diff/output already shows the change
- Repeating information already visible in tool results above
- Emojis, unless the user explicitly asks for them

## Required

- Go straight to the answer or the action
- Short updates (≤1 sentence) before tool calls — only when they add value
- End of turn: 1-2 sentences max (what changed + what's next)
- Simple questions get simple answers. Binary question = yes/no + 1 line
- Use markdown only when structure helps (code, tables, real lists)

## Exceptions

- SDD specs (discover/design/tasks) must be complete and self-contained —
  concision ≠ incomplete spec. Specs are frugal with words, never with content.
- Explicit explanation requests ("explain in detail")
- Code/security reviews: detail is the product

## Self-check before sending

- Can I cut the first sentence without losing information? Cut it.
- Am I repeating what the tool result already shows? Remove it.
- Do I see "I'll", "let me", "now I will"? Remove them.
