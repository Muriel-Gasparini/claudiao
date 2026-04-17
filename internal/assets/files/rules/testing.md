# Rules — Unit tests, coverage, and integrity

## Philosophy (rule zero)

Tests exist to **find bugs**, not to **pass**.
A test that cannot fail is not a test — it is decoration.

Before writing any test, answer:
- "What real bug would this test catch?"
- "If I intentionally break the logic, does this test fail?"

If the answer is "none" or "no", rewrite or delete the test.

## Required

- Every new/changed piece of relevant code ships with **unit tests**.
- The suite must maintain **>= 80% coverage** (line/statement — use the project's convention).
- Do not mark a task complete without:
  - passing tests
  - coverage >= 80% (or explicit exception agreed with the user)
  - integrity checklist satisfied (below)

## Ensuring coverage >= 80%

Preference (in order):
1) **Use the project's existing coverage command** (package manager scripts, Makefile, etc.)
2) If none exists, add/configure a canonical way to collect coverage. Examples:
   - Python/pytest: `pytest --cov --cov-report=term-missing --cov-fail-under=80`
   - Jest: configure `coverageThreshold` in jest config
   - Vitest: configure equivalent thresholds
   - Go: `go test ./... -coverprofile=coverage.out` (and check the %)

**Coverage is not quality.** 80% is a floor, not a target. High coverage with
soft asserts is worse than low coverage with strong asserts — it breeds false confidence.

Effort priority:
1. Critical business paths — strong tests, no mocks
2. Error branches — tested with real input that triggers the error
3. Edge cases — boundaries, concurrency, dirty data
4. Happy path — last (easiest and least informative)

## Mocks — strict rules

**Mock only what you do NOT control:**
- External APIs (third-party HTTP, paid services)
- Clock (`Date.now`, timers) when the test depends on time
- Randomness (`Math.random`, UUID) when determinism is required
- Heavy I/O irrelevant to the SUT (System Under Test)

**Do NOT mock:**
- The code itself that is being tested
- Pure functions of your own project — call them for real
- Database in integration tests — use a real DB (container, in-memory SQLite, fixture)
- Internal layers just to "simplify" — if you need many mocks, the design is wrong; refactor

**Signs of mock abuse (red flags):**
- More than 3 mocks in a single test
- A mock returns exactly what the test asserts as output (tautology)
- The test passes even if you delete the real implementation
- Assertions only check that the mock was called, not the observable effect

## Assertions that matter

Test **observable behavior**, not implementation:

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

## Mental mutation test (required)

For every function under test, brainstorm 3 ways the code could be wrong:
1. Inverted condition (`>` becomes `>=`, `if` becomes `if not`)
2. Off-by-one / wrong boundary
3. Early return / skipped path

Make sure at least one test would fail under each mutation.
If none catches it, **tests are missing** — high coverage with weak asserts does not help.

## Integrity checklist (apply per feature)

- [ ] 1 happy-path test with realistic data
- [ ] 1 error / invalid-input test (null input, wrong type, out of range)
- [ ] 1 edge case test (boundary: 0, 1, max, empty, duplicate)
- [ ] Mental mutation check passes (3 mutations → 3 failing tests)
- [ ] No mocks of internal project code
- [ ] Assertions validate effect, not just invocation
- [ ] Test data would force obvious bugs to surface (does not coincide with impl)
- [ ] Deterministic tests (no reliance on real time/order/randomness)

## Forbidden

- A test that only exists to raise coverage
- `skip`/`xit`/`.only` committed to the repo
- Comments like "TODO: improve this test"
- `try/catch` in a test that swallows the failure silently
- Test data that coincides with the implementation by accident
- Mocking the SUT or its direct internal dependencies
- Tests that pass without the implementation existing

## Evidence

At the end of Implement/Ship, record in the artifact:
- command(s) executed
- result (pass/fail)
- coverage obtained
- integrity checklist ticked
- any mocks used + justification (what and why)
