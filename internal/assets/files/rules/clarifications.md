# Rules — Proactive questions & edge cases

## When to ask

Any time there is relevant ambiguity, **ask proactively** before moving on.

Consider something "ambiguous" when you lack (examples):
- exact goal / expected outcome
- scope (what is in and what is NOT in)
- behavior on errors and invalid states
- business rules (priority, ordering, concurrency, authorization)
- performance / latency / volume
- backward compatibility (migrate data? API version?)
- UX / flows (when UI is involved)

## How to ask — MUST use AskUserQuestion

**ALWAYS** use the `AskUserQuestion` tool when asking the user.
**NEVER** ask questions as plain text, numbered lists, or bullets in chat.

### Tool schema (real API)

```
AskUserQuestion({
  questions: [          // 1-4 questions per call (maxItems: 4)
    {
      question: string,    // Full question text, ends with ?
      header: string,      // REQUIRED, max 12 chars. e.g. "Stack", "Database"
      multiSelect: boolean,// REQUIRED. true=checkbox, false=select
      options: [           // 2-4 options (minItems: 2, maxItems: 4)
        {
          label: string,       // 1-5 words, concise
          description: string  // REQUIRED. Explains the option
        }
      ]
    }
  ]
})
```

### Critical rules

1. **Do NOT include an "Other" option** — the system adds one automatically.
2. **2-4 options per question** — never 5+, the schema rejects it.
3. **header max 12 chars** — required. e.g. "Stack", "Auth", "Scope".
4. **description is required** on every option — always explain.
5. **multiSelect is required** — not optional, always declare it.
6. **1-4 questions per call** — group related ones.
7. Only when the answer is genuinely free-form (a name, a description), plain text is acceptable — rare.

### Examples

**Good** — stack (2 questions, 3 options each):
```
AskUserQuestion({
  questions: [
    {
      question: "What is the project's technology stack?",
      header: "Stack",
      options: [
        { label: "Node.js", description: "JavaScript/TypeScript backend" },
        { label: "Python", description: "FastAPI, Django, or Flask" },
        { label: "Go", description: "Standard library or a framework" }
      ],
      multiSelect: false
    },
    {
      question: "Which primary database?",
      header: "Database",
      options: [
        { label: "PostgreSQL", description: "Relational, ACID compliant" },
        { label: "MongoDB", description: "Document store, flexible schema" },
        { label: "SQLite", description: "Embedded, serverless" }
      ],
      multiSelect: false
    }
  ]
})
```

**Good** — edge cases with checkbox (max 4 options):
```
AskUserQuestion({
  questions: [{
    question: "Which scenarios must the MVP cover?",
    header: "Edge cases",
    options: [
      { label: "Invalid input", description: "Empty fields, wrong types, oversize" },
      { label: "Concurrency", description: "Two users editing the same resource" },
      { label: "External failure", description: "3rd-party API timeout or 5xx" },
      { label: "All of the above", description: "Cover every scenario in the MVP" }
    ],
    multiSelect: true
  }]
})
```

**Bad** — do NOT do this:
```
I have a few questions:
1. What stack do you use?
2. Which database?
3. How do you run tests?
```

## Built-in tools to use in SDD

### TodoWrite — Visible progress

Use `TodoWrite` to show progress to the user during implementation:

```
TodoWrite({
  todos: [
    { content: "Implement POST /invites endpoint", status: "completed", activeForm: "Implementing POST /invites endpoint" },
    { content: "Add email validation", status: "in_progress", activeForm: "Adding email validation" },
    { content: "Write integration tests", status: "pending", activeForm: "Writing integration tests" }
  ]
})
```

TodoWrite rules:
- `content` (imperative) and `activeForm` (gerund) are REQUIRED
- Exactly 1 item `in_progress` at a time
- Mark `completed` IMMEDIATELY when done (never batch)
- Statuses: `pending`, `in_progress`, `completed`

### Grep — Code search

**ALWAYS** use the `Grep` tool to search code. **NEVER** invoke `grep` or `rg` via Bash.

```
Grep({ pattern: "createInvite", type: "ts", output_mode: "content", "-n": true })
```

### Task — Sub-agents

Use `Task` to delegate autonomous sub-agent runs for complex searches:

```
Task({
  description: "Find auth patterns",
  prompt: "Locate all authentication middleware…",
  subagent_type: "Explore"  // or "general-purpose"
})
```

## Edge cases: minimum checklist

Always evaluate and/or ask about (via AskUserQuestion):
- empty / null / missing inputs
- boundaries (0, 1, maximum, very large)
- duplicates / idempotency
- concurrency / races
- permissions / authz
- external failures (network / DB / timeouts)
- data consistency (partially updated records)
- i18n / formatting (dates / decimals) when relevant

## When AskUserQuestion is unavailable

`AskUserQuestion` is required by default. When the tool is **not present in
the current runtime** (subagent context, sandbox, restricted harness), the
agent MUST NOT silently fall back to writing Assumptions. That is the same
failure mode this rule is designed to prevent.

### The only acceptable fallback

**The orchestrator is the single point of contact with the user.** Every
subagent or tool-restricted runtime surfaces its questions upward and waits.

Protocol:

1. The agent emits a **structured question block** back to its parent (the
   orchestrator). The block uses the same shape as `AskUserQuestion` so the
   orchestrator can replay it verbatim:

   ```
   [PENDING_USER_QUESTIONS]
   {
     "from": "sdd-architect",
     "blocking": true,
     "questions": [
       {
         "question": "…?",
         "header": "Auth",
         "multiSelect": false,
         "options": [
           { "label": "…", "description": "…" },
           { "label": "…", "description": "…" }
         ]
       }
     ]
   }
   [/PENDING_USER_QUESTIONS]
   ```

2. The subagent then **stops** — it does not produce a partial spec, does
   not write Assumptions, does not invent answers.

3. The orchestrator collects the block, asks the user with
   `AskUserQuestion` (or plain structured text if it too lacks the tool),
   then re-invokes the subagent with the answers injected into the prompt.

4. The subagent resumes from the answers and continues. It MUST validate
   that every answer it needed is actually present before proceeding.

### Assumptions are a last resort

`Assumptions` in a spec are **only** acceptable when:

- The user explicitly said "just go, don't ask" and accepted unknown risk, **and**
- Each assumption is tagged `⚠️ unvalidated` with: what was assumed, the
  alternative considered, the risk of being wrong, and the owner to confirm.

`Assumptions` are never a substitute for the interview just because a tool
is missing. If the interview cannot happen, the phase is **not Ready**.

### Orchestrator responsibility

The orchestrator:
- Treats subagent output containing `[PENDING_USER_QUESTIONS]` as a blocking
  handoff. It does not discard, paraphrase, or fold it into its own prose.
- Asks the user, using `AskUserQuestion` when available; otherwise renders
  the same block as structured plain text following the same 2-4 option
  rule, and waits.
- Re-spawns or continues the subagent with the user's answers explicitly
  included in the next prompt.
- Never fills in answers on behalf of the user, even to "unblock".

## Assumptions (general)

If you must proceed without an answer (user explicitly opted out of the
interview), declare explicit **Assumptions** with the tagging above and
surface them at the top of the spec. Ask the user to confirm afterwards.
