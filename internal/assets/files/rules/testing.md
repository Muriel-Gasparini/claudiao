# Rules — Testing

Tests exist to **find bugs**, not to **pass**. A test that cannot fail is not a
test — it is decoration. This file is a first-class quality gate: any new or
changed code MUST be evaluated against it before declaring Ready.

## Philosophy (rule zero)

Before writing any test, answer:
- "What real bug would this test catch?"
- "If I intentionally break the logic, does this test fail?"

If the answer is "none" or "no", rewrite or delete the test.

Tests are the **executable specification** of your system. When they pass, the
system behaves as specified. When someone reads them, they learn how to use
the code. Treat them with the same care as production code.

## Testing pyramid

Shape of a healthy suite, bottom to top:

1. **Unit tests** — most tests. Fast (<100 ms each). No I/O. Test pure logic.
2. **Integration tests** — significant. DB, queues, HTTP handlers with real adapters.
3. **Contract tests** — per external interface. Prevent breaking consumers.
4. **E2E / system tests** — few. Critical user journeys only. Slow and fragile.
5. **Smoke tests** — handful. Run post-deploy against prod. Fail loud.

**Ice-cream cone** (many e2e, few unit) is an anti-pattern: slow, flaky,
expensive to maintain.

## Required

- Every new/changed piece of relevant code ships with **unit tests**.
- Suite maintains **>= 80% line coverage** (use the project's convention).
- **Diff coverage** on new code >= 80% (use `diff-cover` or equivalent).
- Tests pass, coverage target met, **integrity checklist** satisfied (below).
- Any bug fix ships with a **regression test** that would have caught the bug.
- Tests are committed **in the same PR** as the code. Never later.

## Coverage

### Target

- **80% line floor.** Not a target — a target makes people game it.
- **Branch coverage** measured when supported; branches matter more than lines for conditionals.
- **Diff coverage** enforced per PR — new code has no excuse.

### How to collect

1. Use the project's existing coverage command (package manager scripts, Makefile).
2. If none exists, add one:
   - Python/pytest: `pytest --cov --cov-report=term-missing --cov-fail-under=80`
   - Jest: `coverageThreshold` in jest config
   - Vitest: equivalent thresholds
   - Go: `go test ./... -coverprofile=coverage.out` + threshold script
   - JVM: JaCoCo with per-module rules

### What coverage is not

**Coverage is not quality.** 80% with soft asserts is worse than 60% with strong
asserts — it breeds false confidence. Mutation score is a better quality signal
(see below).

### What to exclude

- Generated code (protobuf stubs, ORM migrations)
- `main()` / entry points that only wire up dependencies
- Vendored code

### Effort priority

1. Critical business paths — strong tests, no mocks
2. Error branches — tested with real input that triggers the error
3. Edge cases — boundaries, concurrency, dirty data
4. Happy path — last (easiest and least informative)

## Test structure

### Arrange / Act / Assert (Given / When / Then)

Every test has three phases with visual separation:

```
// Arrange (Given)
user := newUser(t, WithRole("admin"))

// Act (When)
err := api.DeleteUser(ctx, user.ID)

// Assert (Then)
require.NoError(t, err)
require.False(t, api.UserExists(ctx, user.ID))
```

### One behavior per test

Not "one assert" — **one behavior**. A test can have multiple asserts verifying
the same behavior. But if you are testing two independent things, split them.

### Descriptive names

Bad: `TestUser1`, `TestCreate`.
Good: `TestCreateUser_WhenEmailExists_ReturnsConflict`,
`test_parse_rejects_negative_amount`.

Names describe **input + expectation**, not the function under test.

## Isolation & determinism

- **Tests do not share mutable state.** No global vars, no shared DB rows across tests unless explicitly scoped (transaction rolled back, schema cleaned).
- **Order-independent.** Running tests in random order must not break them. Many frameworks support `--shuffle`; enable it.
- **Parallel-safe.** Unit tests run in parallel by default. Integration tests declare shared resources and coordinate.
- **No flaky tests.** A retry is a bandage, not a fix. Quarantine + fix within a deadline; delete if untouched.

### Freeze time & randomness

- Wrap `time.Now()` / `Date.now()` / `datetime.now()` behind a `Clock` interface; inject a `FakeClock` in tests.
- Seed random generators explicitly; use a fixed seed in tests.
- Never `sleep` in tests. Wait on conditions with a timeout.

### Network

- Unit tests do not touch the network. Disable it (Go: `httptest`, Node: `nock`, Python: `responses`).
- Integration tests talk to **known endpoints** under your control (testcontainers, local sandbox, WireMock).

## Test data

### Factories & builders

- Use factory functions / builders (`NewUser(t, opts...)`) over hand-rolled fixtures. Express intent: `newUser(t, WithRole("admin"), WithVerified())`.
- Prefer **Faker**-generated realistic data over `"foo"`/`"bar"`. Obvious bugs (name-length overflow, email validation) surface easily.
- Tests create the minimum data they need. Shared "mega-fixtures" hide coupling.

### Seed & reset

- Each test starts from a known state. Reset via transaction rollback (fast), truncation (medium), or recreate (slow — last resort).
- Never depend on prior test output.

### No prod data in tests

Use synthetic data, or scrubbed/anonymized snapshots. Even in dev.

## Mocks — strict rules

### Mock only what you do NOT control

- External APIs (third-party HTTP, paid services)
- Clock (`Date.now`, timers) when the test depends on time
- Randomness (`Math.random`, UUID) when determinism is required
- Heavy I/O irrelevant to the SUT (System Under Test)

### Do NOT mock

- The code itself that is being tested
- Pure functions of your own project — call them for real
- Database in integration tests — use a real DB (container, in-memory SQLite, fixture)
- Internal layers just to "simplify" — if you need many mocks, the design is wrong; refactor

### Test doubles spectrum (pick the right one)

- **Dummy**: object passed but not used.
- **Stub**: returns canned values.
- **Fake**: working lightweight implementation (in-memory DB, fake clock).
- **Spy**: records calls, returns canned values.
- **Mock**: expects specific calls; fails on mismatch.

**Prefer fakes over mocks.** Mocks tightly couple tests to implementation.

### Signs of mock abuse (red flags)

- More than 3 mocks in a single test
- A mock returns exactly what the test asserts as output (tautology)
- The test passes even if you delete the real implementation
- Assertions only check that the mock was called, not the observable effect

## Assertions that matter

Test **observable behavior**, not implementation.

Good:
- Function return value
- Persisted state (read back from the DB later)
- Emitted event/message (consume the queue)
- Specific error thrown with the expected message
- Observable side effect in the test world

Bad:
- `expect(mock).toHaveBeenCalled()` alone
- `assert response.status == 200` without checking the body
- `assert True` / `assert result is not None`
- Whole-object snapshot without understanding what changed

### Rich matchers

Use matchers that produce helpful failure messages: `expect.objectContaining`,
`MatchObject`, `require.ElementsMatch`. Prefer them over ad-hoc equality checks
that print "expected A, got B" with no diff.

## Mutation testing

### Mental mutation (required, per function)

Brainstorm 3 ways the code could be wrong:
1. Inverted condition (`>` → `>=`, `if` → `if not`)
2. Off-by-one / wrong boundary
3. Early return / skipped path

At least one test must fail under each mutation. If none does, tests are missing.

### Actual mutation tools (recommended for critical modules)

- JS/TS: **Stryker**
- Java/Kotlin: **PIT**
- Python: **mutmut**, **cosmic-ray**
- Go: **go-mutesting**, **gremlins**

Run periodically (slow). Track mutation score for business-critical modules.

## Property-based testing

For pure functions and data transformations, express invariants and let the
framework generate adversarial inputs:

- JS/TS: **fast-check**
- Python: **hypothesis**
- Go: `testing/quick`, **rapid**
- Haskell/JVM: **QuickCheck**, **ScalaCheck**, **jqwik**

Examples of invariants: `reverse(reverse(xs)) == xs`, `sort(xs)` is sorted,
`encode` then `decode` is identity, `add(a, b) == add(b, a)`.

Property tests find edge cases humans miss (empty inputs, unicode, negative
zero, overflow).

## Fuzzing

For parsers, validators, protocol decoders, and any function that consumes
untrusted bytes:

- Go: `testing.F` + `F.Fuzz` (built-in)
- C/C++: **libFuzzer**, **AFL++**
- Rust: **cargo-fuzz**
- Multi-language: **OSS-Fuzz** (continuous)

Fuzzing is a security and reliability measure. Crash = bug. Every crash gets a
regression test.

## Snapshot testing

- Use sparingly for **stable, reviewed outputs** (rendered markup, serialized API responses).
- **Review every diff** — do not auto-accept. Auto-update without review turns snapshots into rubber stamps.
- Avoid snapshots of large structured data — write focused asserts instead.

## Golden files

- Commit known-good outputs (e.g. rendered CLI output, formatted SQL).
- Refresh with `go test -update` or equivalent **plus a reviewer**.

## Integration tests

- **Real adapters**: real DB in a container (Testcontainers), real Redis, real HTTP server.
- Each test in its own transaction (rolled back) or its own schema (recreated fast).
- **Never** mock the DB in an integration test that exists to test DB behavior.
- Cover: migrations apply cleanly, queries match schema, constraints enforce, indexes exist.

## E2E / system tests

- **Few.** 1–5 critical journeys. They are slow, flaky-prone, and expensive to maintain.
- Tools: **Playwright**, **Cypress**, **Selenium**. Prefer Playwright for modern web.
- Target **data-testid** attributes, not CSS/text (less fragile).
- Run in a real browser, headless in CI.
- Retry once at most; a flaky e2e is a bug to fix, not to paper over.

## Contract tests

For services that depend on each other, enforce the contract explicitly:

- **Consumer-driven contracts**: **Pact**, **Spring Cloud Contract**.
- Consumer publishes expectations; provider verifies. Breaking the contract breaks CI before deploy.
- Essential for microservices and shared internal APIs.

## Performance tests

(Cross-refer `performance.md` for details.)

- Every latency-sensitive endpoint has a load test.
- Baseline committed to the repo; CI flags regressions > N%.
- Tools: **k6**, **Locust**, **Gatling**, **wrk**.

## Security tests

(Cross-refer `security.md`.)

- Negative authz tests: another tenant's ID, expired role, revoked token.
- Input boundary tests: injection vectors, oversize payloads, malformed bodies.
- Auth flow tests: CSRF token validation, session rotation on privilege change, account enumeration equality.
- Dependency scans integrated as "tests" in CI.

## Accessibility tests

For any UI:

- Automated: **axe-core** (`@axe-core/playwright`, `jest-axe`) — zero violations on main flows.
- Keyboard navigation test: can the user complete the journey with Tab/Enter only?
- Screen reader labels for interactive elements (verify via automated audit).
- Color contrast ≥ WCAG AA.

## Visual regression

- **Percy**, **Chromatic**, **Playwright screenshots** for UI-heavy projects.
- Commit baselines; review every visual diff.
- Scope tight — full-page screenshots produce noisy diffs; component-level is cleaner.

## Async / timing

- **Never** `sleep(N)` to wait for async work. Wait for a **condition** with a timeout (`waitFor`, `eventually`).
- Event-driven code: drive via explicit events, not arbitrary delays.
- Deadlines/timeouts in tests should be generous enough not to flake but tight enough to catch real stalls.

## Flakiness

Causes are almost always:
1. **Timing** (race conditions, arbitrary sleeps)
2. **Shared mutable state** (global vars, DB rows leaking between tests)
3. **Order dependence** (test B relies on state from A)
4. **External services** (network, time, randomness)
5. **Resource limits** (ports, file handles, DB connections)

Protocol:
- First failure: investigate. Root-cause before anything else.
- **Quarantine** if fix is not immediate. Ticket with deadline.
- **Delete** quarantined tests that miss deadline.
- Never "just re-run CI" as a fix.

## Suite performance

- Unit suite ≤ 2 minutes ideal, 10 minutes hard cap.
- **Parallelize** aggressively (`-parallel`, `-p`, `pytest-xdist`).
- **Shard** in CI for very large suites.
- Cache dependencies between CI runs.
- Tag slow tests (`@slow`, `//go:build integration`); skip locally, run in CI.

## CI integration

- Run on **every PR and every push to main**.
- **Fail fast** — surface failures in < 5 min where possible.
- Publish **JUnit XML / coverage report** as artifacts.
- **Required checks** on main branch: tests, coverage floor, lint.
- CI environment mirrors prod OS + language version.

## Test environment & seed

- Separate env from dev/prod: separate DB, separate cloud project, separate secrets.
- Seed reproducible: `make test-seed` creates a known state.
- Migrations tested: start empty DB, run all migrations, assert schema.

## Cleanup

- `t.Cleanup(fn)` / `afterEach` / teardown to release resources.
- No leaked goroutines, file descriptors, DB connections, processes after a test.
- Assert clean state if applicable (`goleak` in Go).

## Test anti-patterns

Avoid:

- **Ice cream cone**: mostly e2e, few unit. Slow, flaky, expensive.
- **God test**: covers 20 scenarios in one function; hard to read and debug.
- **Whitebox testing internals**: asserting on private state instead of behavior. Tests break on refactor even when behavior is correct.
- **Logic in tests**: loops, conditionals, calculations. Tests should read like a spec, not a program.
- **Obscure test**: unclear what is being tested without reading the entire file.
- **Conditional asserts** (`if x { assert.Equal(...) }`) — the test always "passes" when the branch is not taken.
- **Ignored tests** (`.skip`, `xit`, `@Ignore`) committed without a ticket.
- **Testing the framework** (asserting `.map()` works). Trust the platform.
- **Shared random seed across tests** without seeding explicitly per test.

## TDD / BDD

- **TDD**: Red → Green → Refactor. Valuable for well-specified logic (parsers, calculators, validation).
- **BDD**: Given/When/Then naming; business-readable tests. Good for acceptance criteria on user-facing behavior.
- Neither is mandatory; both are **tools**. Use when they pay off.

## Tests as documentation

- Readers learn your API from your tests. Write them so a newcomer understands usage.
- Keep examples minimal, realistic, and focused.
- A `README` referencing `*_test.go` examples is often more accurate than hand-written docs.

## Regression tests

- Every bug fix ships with a test that would have caught the bug.
- Keep these tests forever — rename them if the bug doesn't have a name, but do not delete.

## AI-specific (LLMs)

Testing code that calls LLMs is different:

- **LLM output is non-deterministic.** Do not assert exact text. Assert:
  - Structure (JSON validates against schema)
  - Presence of key facts (substring, embedding similarity)
  - Absence of red flags (refusals, leaked prompts, unsafe content)
- **Mock LLM in unit tests** with canned responses. **Use real LLM in contract/eval tests** at a slow cadence.
- **Eval sets**: maintain a curated set of prompts + expected structural outputs. Run on every model/prompt change. Track pass rate over time.
- **Test prompt injection defenses**: include adversarial inputs in the eval set.
- **Regression tests** for prompt changes: did we break a capability we had?

## Evidence (record per Implement/Ship)

- Command(s) executed
- Result (pass/fail)
- Coverage obtained (total + diff)
- Integrity checklist ticked
- Any mocks used + justification (what and why)
- Flakes observed + tickets opened
- Mutation score (if run)

## Integrity checklist (apply per feature)

- [ ] 1 happy-path test with realistic data
- [ ] 1 error / invalid-input test (null, wrong type, out of range)
- [ ] 1 edge case test (boundary: 0, 1, max, empty, duplicate)
- [ ] 1 concurrency test if the code is concurrent
- [ ] 1 authorization-negative test if the code enforces authz
- [ ] Mental mutation check passes (3 mutations → 3 failing tests)
- [ ] No mocks of internal project code
- [ ] Assertions validate effect, not just invocation
- [ ] Test data would force obvious bugs to surface (does not coincide with impl)
- [ ] Deterministic (no real time/order/randomness dependence)
- [ ] Isolated (no shared mutable state; parallel-safe)
- [ ] Fast enough to run in the unit tier (<100 ms) or tagged `integration`
- [ ] Names describe input + expectation
- [ ] Regression test attached for any bug fixed

## Forbidden

- A test that only exists to raise coverage
- `skip` / `xit` / `.only` committed to the repo
- Comments like "TODO: improve this test"
- `try/catch` in a test that swallows the failure silently
- Test data that coincides with the implementation by accident
- Mocking the SUT or its direct internal dependencies
- Tests that pass without the implementation existing
- Auto-retrying flaky tests in CI as a "fix"
- Sleep-based synchronization
- Sharing DB state across tests without explicit setup/teardown
- Asserting on whole-object snapshots without understanding the diff
- Logic (loops/conditionals) in tests
- Disabling a test without an open ticket + deadline
- Running tests against production
