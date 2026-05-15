# Rules — Proactive questions & edge cases

## When to ask

Ask **only when there is real ambiguity** — exact goal, scope, error
behavior, business rules, performance, backward compatibility, UX. If
the request is clear, don't ask. Group 1-3 questions in a single
`AskUserQuestion` call. No mandatory rounds.

## How to ask — MUST use AskUserQuestion

Use the `AskUserQuestion` tool. Never plain text, numbered lists, or
bullets in chat.

### Schema

```
AskUserQuestion({
  questions: [          // 1-4 per call (maxItems: 4)
    {
      question: string,    // ends with ?
      header: string,      // REQUIRED, max 12 chars (e.g. "Stack")
      multiSelect: boolean,// REQUIRED
      options: [           // 2-4 options
        { label: string, description: string }  // 1-5 words; description REQUIRED
      ]
    }
  ]
})
```

### Critical rules

- **Do NOT include an "Other" option** — system adds it automatically.
- **2-4 options** per question; never 5+ (schema rejects).
- **header max 12 chars**; **description required** on every option.
- **multiSelect** is required (boolean, not optional).
- **1-4 questions per call**; group related ones.
- Free-form answers (a name, a description) → plain text is acceptable, rare.

### Example

```
AskUserQuestion({
  questions: [{
    question: "Which scenarios must the MVP cover?",
    header: "Edge cases",
    multiSelect: true,
    options: [
      { label: "Invalid input", description: "Empty fields, wrong types, oversize" },
      { label: "Concurrency", description: "Two users editing the same resource" },
      { label: "External failure", description: "3rd-party API timeout or 5xx" }
    ]
  }]
})
```

## Edge cases — minimum checklist to evaluate or ask about

- empty / null / missing inputs
- boundaries (0, 1, maximum, very large)
- duplicates / idempotency
- concurrency / races
- permissions / authz
- external failures (network / DB / timeouts)
- data consistency (partially updated records)
- i18n / formatting (dates / decimals) when relevant

## When AskUserQuestion is unavailable

In subagent or restricted-tool runtimes, the agent MUST NOT silently
fall back to writing Assumptions. Protocol:

1. Emit a `[PENDING_USER_QUESTIONS]` block back to the orchestrator
   (same shape as `AskUserQuestion`):

   ```
   [PENDING_USER_QUESTIONS]
   { "from": "<agent>", "blocking": true, "questions": [ … ] }
   [/PENDING_USER_QUESTIONS]
   ```

2. **Stop.** Do not write a partial spec or invent answers.

3. The orchestrator asks the user (with `AskUserQuestion` if available)
   and re-invokes the subagent with the answers in the prompt.

The orchestrator never fills in answers on the user's behalf. Treating
`[PENDING_USER_QUESTIONS]` as anything other than a blocking handoff
is a defect.

## Assumptions — last resort

Acceptable **only** when the user explicitly opted out of the interview
("just go, don't ask"). Each assumption tagged `⚠️ unvalidated` with:
what was assumed, the alternative considered, the risk of being wrong.
Surface them at the top of the response and ask the user to confirm
afterwards.
