# Rules — SDD Quality Gates

## Philosophy

"Ready" is a declaration of **evidence**, not an opinion. The person (or
agent) saying "Ready: yes" is staking their credibility on having verified
the criteria below — and on having done so against the actual artifact,
not against a feeling.

Every optimistic Ready that later turns out false is a **gate failure**,
and costs more than a hundred conservative "Ready: no" declarations.

## Hard rules

- **The phase agent does not declare itself Ready.** It submits the spec.
  The **orchestrator** runs the rubric and declares Ready (or delegates to
  the reviewer/release-manager agent for later phases).
- **Self-review is not optional.** The orchestrator runs the rubric
  adversarially — looking for reasons it is NOT ready.
- **Blockers and Majors are never compatible with Ready: yes.** Fix first.
- **Minors** may ship; they are recorded in a "Deferred" section of the spec.
- **Every Ready decision produces a written report** in the format below.

## Never declare "Done/Ready" without:

- A spec sufficient for the phase (no `…`, `TBD`, unfilled headers, unlabeled assumptions)
- Clear, **measurable** acceptance criteria (not "fast", not "easy to use")
- Documented edge cases with expected behavior
- Relevant tests passing (when the phase produces code)
- Unit coverage >= 80% (when the phase produces code)
- **Rubric run against the artifact**, with findings written out
- **Zero Blockers, zero Majors**

## Ready report — required format

Every phase ends with either:

```
Ready: yes

Evidence
- Phase: <discover|design|tasks|implement|review|ship>
- Artifact: <path to spec/code>
- Rubric: <which checklist was applied>
- Reviewer agent invoked: <yes|no, reason>
- Blockers: 0
- Majors: 0
- Minors: <n, each listed with action owner>
- Open questions: 0 blocking
- Coverage: <%, when code phase>
- Evidence links: <paths to sections / CI runs / logs>

Checks performed
- <each checklist item from the rubric, marked ✓>
```

Or:

```
Ready: no

Reason
- Blockers: <n, listed with path to offending section>
- Majors: <n, listed with path>
- Missing rubric items: <listed>
- Required fixes before retry: <bulleted>
```

An unformatted "Ready: yes" or "looks good" is rejected at review time.

## Severity definitions

- **Blocker** — factually wrong, security/privacy risk, broken contract,
  missing acceptance criterion, unstated assumption that changes behavior.
  Must fix before Ready.
- **Major** — ambiguity that a reasonable implementer would resolve wrongly;
  edge case without stated behavior; missing NFR number; untested critical
  path. Must fix before Ready.
- **Minor** — wording, ordering, formatting, nit. May ship; record in a
  "Deferred" section with an owner.

When in doubt, rank upward. False negatives are worse than false positives.

## "Ready" per phase — rubric

### Discover — `01-discover.md`

- [ ] Vision and goals clear enough for an outsider to explain
- [ ] Non-goals explicit with reason
- [ ] User personas defined with a real use case each
- [ ] MVP defined, minimal, with acceptance criteria (measurable)
- [ ] At least 2-3 concrete scenarios with realistic data
- [ ] Edge cases listed with **expected behavior** (not just a name)
- [ ] Constraints and risks recorded
- [ ] Open questions: 0 blocking (non-blocking moved to a section)
- [ ] Assumptions tagged `⚠️ unvalidated` if any
- [ ] Handoff section filled for the Architect

### Design — `02-design.md`

- [ ] Inherited context from Discover present and correct
- [ ] Every decision has alternatives documented with pros/cons
- [ ] Contracts (API / events) with request + response + error JSON examples
- [ ] Data model / schema with types, indexes, constraints, and reasoning
- [ ] NFRs with concrete numbers (latency p95/p99, volume, error budget)
- [ ] Error flow documented, not only happy path
- [ ] Rollout / migration plan with rollback
- [ ] Security considerations (cross-ref `rules/security.md`)
- [ ] Performance considerations (cross-ref `rules/performance.md`)
- [ ] Handoff section filled for the Dev Lead

### Tasks — `03-tasks.md`

- [ ] Inherited context from Discover + Design present
- [ ] Tasks small (≤ ~4h each) and independently verifiable
- [ ] Each task: description, dependencies, acceptance, edge cases, tests, notes
- [ ] Execution order explicit and justified
- [ ] Definition of Done defined for the feature
- [ ] No task says "as needed" or "etc." — all concrete
- [ ] Handoff section filled for the Implementer

### Implement — `04-implementation.md`

- [ ] All tasks from `03-tasks.md` done or explicitly deferred
- [ ] Tests passing: command + result recorded
- [ ] Coverage ≥ 80%: number recorded
- [ ] Integrity checklist from `rules/testing.md` ticked
- [ ] Any divergence from spec recorded with reason
- [ ] Commands, decisions, and outputs logged in the implementation diary
- [ ] No commented-out code, no `TODO` without a ticket, no dead code
- [ ] Handoff section filled for the Reviewer

### Review — `05-review.md`

- [ ] All specs read before reviewing
- [ ] Findings classified must-fix / should-fix / nit
- [ ] Each finding: what is wrong, why it matters, how to fix, spec reference
- [ ] Security rubric from `rules/security.md` run against the diff
- [ ] Performance rubric from `rules/performance.md` run where applicable
- [ ] Testing integrity checklist from `rules/testing.md` verified
- [ ] Code quality checklist from `rules/code-quality.md` verified
- [ ] No must-fix items outstanding before Ready

### Ship — `06-ship.md`

- [ ] Review report has no outstanding must-fix items
- [ ] Tests passing with coverage ≥ 80% on target branch
- [ ] Feature flag configured (if applicable)
- [ ] Rollout plan defined
- [ ] Rollback plan documented and rehearsed mentally
- [ ] Observability (metrics, logs, alerts) in place
- [ ] Release notes written
- [ ] On-call informed

## Anti-patterns — rejected at gate

- "Ready: sim" without the Evidence block.
- "I think we're good" / "should be fine" as a Ready signal.
- Moving open questions into "Assumptions" to clear the checklist.
- Marking Minor what is actually a Major ("downgrading to ship faster").
- Declaring Ready on a self-written spec without running the rubric.
- Skipping the rubric because "it's a small change".
- "Tests pass locally" — evidence is CI runs, not claims.
- Coverage numbers reported without the tool/command that produced them.

## When the user says "review"

If the user asks you to review after you already declared Ready and you
find issues: **the prior Ready was a defect**. Acknowledge it, fix the
process, not just the spec.

"I should have caught this before declaring Ready. Running the rubric
properly this time" is the right response. Not "good catch".
