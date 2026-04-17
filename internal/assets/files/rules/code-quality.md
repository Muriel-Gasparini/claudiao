# Rules — Code Quality

Quality is not aesthetics. It is the property that lets the same codebase keep
shipping features safely, five years from now, with engineers who were not
there when it was written. This file is a first-class quality gate.

## Principle

- **Code is read 10× more than it is written.** Optimize for the reader.
- **Explicit beats implicit.** Magic is a bug in disguise.
- **Make illegal states unrepresentable** via the type system. Runtime checks are the fallback, not the first line.
- **Simplicity > cleverness.** Clever code is a bill paid by the next maintainer.
- **The best code is no code.** Every line is a liability: it can break, drift, and cost attention forever.
- **Small, focused, testable.** Each unit does one thing and can be verified alone.

## Naming

- Names reveal intent. `d` ≠ `daysElapsed`.
- **Verbs for functions** (`calculateInvoiceTotal`), **nouns for values** (`invoiceTotal`), **adjectives for booleans** (`isActive`, `hasAccess`, `canEdit`).
- No abbreviations except universally understood (`url`, `id`, `http`).
- No **Hungarian notation** (`strName`, `iCount`). The type system knows.
- No numeric suffixes (`data2`, `userList3`). If you need two, name them for their difference.
- Boolean names are positive (`isEnabled`, not `isNotDisabled`).
- **Consistent vocabulary**: one concept, one word. Don't mix `fetch`/`get`/`retrieve`/`load` for the same operation.
- Constants in `UPPER_SNAKE_CASE` (language idioms apply).

## Functions

- **Do one thing.** If you need "and" in the name, split it.
- **Short.** Target ≤ 30 lines. Hard warning at 50. Not a rigid rule, but a smell.
- **Few parameters.** 0-2 ideal, 3 ok, 4+ suspicious. Group related args into a struct/object.
- **No boolean flags as parameters.** `user.save(true)` — what's `true`? Split into two named functions or use an enum.
- **Return early.** Guard clauses over nested `if` pyramids.
- **Pure where possible**: same input ⇒ same output, no side effects. Push impurity (I/O, time, random) to the edges.
- **Command/Query Separation**: either mutate or return, not both.
- **No output parameters.** Return a value.

## Modules & organization

- **One concept per file.** File name = primary export.
- **Group by feature/domain, not by technical layer.** `/users/*` beats `/controllers + /services + /repositories` top-level.
- **Stable layers**: domain (no imports from infra/UI), application (uses domain), infrastructure (adapters), interface (UI/HTTP/CLI).
- **Explicit public API per module.** Private helpers stay private.
- **Barrel files (`index.ts`) sparingly** — easy to create, easy to create circular imports and bundle bloat.

## Strong typing

### Universal

- **Every public function has explicit parameter and return types.**
- **Make illegal states unrepresentable**: discriminated unions, exhaustive pattern matching, newtype / branded types, enum for closed sets.
- **No stringly-typed code.** `status: "active" | "banned"` beats `status: string`.
- **Parse, don't validate.** At the boundary, parse raw input into a typed domain value; downstream code receives guarantees, not strings.

### TypeScript

- `tsconfig` must have: `strict: true`, `noUncheckedIndexedAccess: true`, `exactOptionalPropertyTypes: true`, `noImplicitOverride: true`.
- **No `any`.** If forced, `// eslint-disable-next-line @typescript-eslint/no-explicit-any -- <reason>`; review rejects un-justified `any`.
- **Prefer `unknown` over `any`** and narrow with type guards.
- **No `// @ts-ignore`** — use `// @ts-expect-error <reason>`; it fails if the line stops being an error.
- **Prefer `satisfies` over `as`.** `as` is an unsafe cast; `satisfies` preserves the inferred type.
- **Discriminated unions** over inheritance. `type Event = Login | Logout | …` with a `kind` tag.
- **Exhaustiveness checks** on unions: `const _: never = value;` in the default branch.
- `readonly` arrays/properties by default; mutation is opt-in.
- Generics with **constraints** (`<T extends Entity>`) — unconstrained generics usually signal missing design.
- Prefer **ES modules**, avoid `namespace`.
- `const` by default; `let` when reassignment is required; `var` never.
- Enable `typescript-eslint` **strict** preset.

### Go

- **Avoid `interface{}` / `any`** except at serialization boundaries.
- **Accept interfaces, return structs.**
- **Small interfaces** (1–3 methods). Define them where they are used, not where they are implemented.
- **Wrap errors** with context: `fmt.Errorf("create user: %w", err)`. Use `errors.Is` / `errors.As` to inspect.
- **Package names** short, lowercase, no underscores, meaningful (`user`, not `utils`).
- Toolchain: `go vet`, `golangci-lint` (include `staticcheck`, `gosec`, `govulncheck`, `errcheck`, `revive`, `gocyclo`, `gocognit`).

### Python

- **Type hints everywhere** (PEP 484 / 604 / 695).
- **`mypy --strict`** or Pyright strict in CI.
- Avoid `Any`; prefer `object` + narrowing, or precise generics.
- Use **`dataclasses` / `attrs` / `pydantic`** for data containers.
- Toolchain: `ruff` (linter + formatter), `black` if you prefer it, `isort` (or ruff rules), `mypy`/`pyright`.

### Java / Kotlin

- **Null safety**: Kotlin built-in, Java via `Optional<T>` + `@Nullable` / `@NonNull` annotations.
- **`final` fields/parameters by default**; immutability over mutability.
- **Avoid raw types**. Always parameterize generics.
- Prefer `record` / `data class` for value types.
- Toolchain: `ErrorProne`, `SpotBugs`, `Detekt`, `ktlint`, Checkstyle.

### Rust

- **Clippy** with `-D warnings` in CI; allow only with justification.
- **`unsafe` blocks** require a comment explaining invariants.
- Prefer **iterators and combinators** over indexed loops.
- `Result<T, E>` over panics in library code.

## Null & errors

- **No `null` in the domain.** Use `Option` / `Maybe` / `Optional` at boundaries; unwrap explicitly.
- **Errors are values.** Return them; don't use exceptions for control flow.
- **Typed errors** (sum types, sealed classes, error enums) over generic `Error` / `Exception`.
- **Wrap with context.** `error: fetching user 42: timeout` beats `error: timeout`.
- **Fail fast.** A surprising state → explicit error, not silent coercion.
- **Do not swallow errors.** `catch` without handling is a bug.
- **Don't log-and-rethrow.** Do one.

## Immutability

- **Default to immutable.** `const`, `readonly`, `final`, `val`, `record`, frozen collections.
- Copy-on-write when modifying shared state.
- Value objects are immutable.
- Mutation is explicit, localized, and justified.

## Clean Code

### DRY, YAGNI, KISS — with nuance

- **DRY** duplication of **knowledge**, not syntax. Two lines that look alike but represent different concepts are fine.
- **YAGNI**: no speculative generality. Build what is needed now; generalize when the second use case arrives.
- **KISS**: the obvious solution beats the clever one — unless you can defend the cleverness with measurements or a well-known constraint.
- **Rule of three**: abstract only on the third occurrence. Two is a coincidence.

### Comments

- Default: **no comments**. Names and structure explain themselves.
- Write a comment only when the **WHY is non-obvious**: a hidden constraint, a subtle invariant, a workaround for a specific bug, behavior that would surprise a reader.
- Never explain the **WHAT** — the code already does that.
- Never reference the current task, PR, or caller ("added for ticket-123", "used by X") — that rots.
- Docstrings on **public API** (parameters, returns, errors, examples).
- Dead code → deleted, not commented out. Git keeps history.

### Readability

- **Vertical density** — related lines close together, unrelated lines separated by a blank line.
- Short lines (80–120 cols).
- Consistent formatting — enforced by a formatter, not by humans.

## SOLID

- **Single Responsibility Principle (SRP)**: a class/module has one reason to change. If a change request touches two unrelated concerns in the same class, split it.
- **Open/Closed Principle (OCP)**: code is open for extension, closed for modification — achieved with polymorphism, strategies, plugins, or configuration, not by editing the old class every time.
- **Liskov Substitution Principle (LSP)**: subtypes honor the contract of their supertype — preconditions no stricter, postconditions no weaker, no surprising exceptions.
- **Interface Segregation Principle (ISP)**: many small interfaces over one fat interface. Consumers depend only on the methods they use.
- **Dependency Inversion Principle (DIP)**: high-level modules depend on abstractions, not on concretions. Inject dependencies; do not `new` them inside constructors.

## Design patterns

### Use when they fit — not because they exist

Patterns are vocabulary, not goals. Overusing patterns creates ceremony without benefit ("FactoryStrategyFacadeBuilder").

### Useful defaults

**Creational**
- **Factory / Builder** for complex construction with variants.
- **Dependency Injection** over service locator / global singletons.
- **Singleton** — prefer module-level instance if the language supports it; avoid true Singletons that hide dependencies.

**Structural**
- **Adapter** to bridge incompatible interfaces (especially at the infra boundary).
- **Decorator** for layered cross-cutting concerns (logging, retry, auth).
- **Facade** to simplify a complex subsystem for its clients.

**Behavioral**
- **Strategy** for interchangeable algorithms.
- **Observer / Pub-Sub** for one-to-many notifications — keep consumers decoupled.
- **State** to replace `switch` on type with polymorphic behavior.
- **Command** when actions are first-class (queued, undoable, logged).
- **Template Method** when steps vary but the outline is fixed.

### Domain patterns

- **Repository** — encapsulate persistence, return domain objects. Keep the domain layer free of ORM types.
- **Service** for operations that don't belong on an entity.
- **Unit of Work** for atomic multi-repository changes.
- **Specification** for composable query predicates.

### Anti-patterns to avoid

- **Singleton everywhere** (hidden global state; kills testability).
- **Service locator** (hides dependencies).
- **Anemic domain model** (all logic in services, entities are just bags of getters).
- **God object** (one class that knows about everything).
- **Utility dumping ground** (`Utils.java`).
- **Feature envy** (method uses another object's data more than its own — move it).

## Architecture

### Hexagonal / Ports & Adapters

- **Domain** at the center. Pure, no framework imports.
- **Ports** (interfaces) are owned by the domain.
- **Adapters** implement ports: HTTP handlers, DB repositories, message consumers.
- The domain does not import the adapters. The adapters import the domain.

### Domain-Driven Design (when complexity warrants)

- **Entities** have identity and lifecycle.
- **Value objects** are immutable, compared by value.
- **Aggregates** enforce invariants across a cluster of entities; one write per aggregate per transaction.
- **Domain events** carry facts about what happened.
- **Ubiquitous language** — the code uses the business vocabulary. If the business says "invoice," the class is `Invoice`, not `BillingRecord`.

### Layering rules

- Dependencies point inward (`infra → app → domain`). The domain never imports from above.
- Cross-cutting concerns (logging, auth, rate-limit) live in middleware/decorators, not scattered.
- Feature folders > technical folders: `/features/invoices/*` beats `/controllers + /services + /repositories`.

## Testable code

Testability is a design property, not an afterthought. If code is hard to test, it is hard to reason about and hard to change.

- **Inject dependencies.** No `new` inside a constructor for a dependency you want to swap in a test.
- **Pure functions in the domain.** Side effects at the edges.
- **Mockable seams**: depend on interfaces, not concrete implementations.
- **No static singletons** for I/O (clock, HTTP, DB). Inject them.
- **No hidden dependencies.** If the code needs something, it asks for it explicitly.

(See `testing.md` for the testing rules themselves.)

## Concurrency

- **Shared nothing when possible.** Prefer message passing (channels, actors, queues) over shared mutable state.
- **Immutable messages.**
- **Bound concurrency** (see `performance.md`).
- **Document threading contract** on every public type: "safe for concurrent use" / "not safe" / "safe with external lock".
- Beware of hidden concurrency: async iterators, streams, background flush tasks.

## Code smells (refactor when you see them)

- **God class / god function** — > ~500 lines, > ~15 methods.
- **Long method** — doing more than one thing.
- **Long parameter list** — introduce a parameter object.
- **Primitive obsession** — `string` for email, money, IDs. Create newtypes.
- **Feature envy** — method uses another object's data more than its own.
- **Data clumps** — the same 3+ fields always travel together. Extract a type.
- **Switch on type** — replace with polymorphism.
- **Temporary field** — field only valid in some states. Split the class or use a proper state type.
- **Middle man** — class only delegates. Flatten.
- **Divergent change** — one class changes for unrelated reasons. Split.
- **Shotgun surgery** — one conceptual change requires edits in many files. Consolidate.
- **Comments explaining bad code** — refactor the code instead.
- **Dead code** — delete it.
- **Speculative generality** — flexibility nobody uses.

## Refactoring moves

- **Extract method / class / variable**
- **Rename** (fear no rename — the IDE handles it)
- **Introduce parameter object**
- **Replace conditional with polymorphism**
- **Replace magic number with named constant**
- **Replace nested conditional with guard clauses**
- **Replace null with Option**
- **Replace boolean flag with two methods**

Small, mechanical, safe. Never mix refactoring with behavior change in the same commit.

## Complexity limits

- **Cyclomatic complexity** per function ≤ 10 (warn at 10, fail at 15).
- **Cognitive complexity** ≤ 15.
- **Function length** ≤ 50 lines (warn at 30).
- **File length** soft cap 500 lines, hard cap 1000.
- **Nesting depth** ≤ 3.
- Enforced in CI via lint rules.

## Formatting & style

- **Formatter is non-negotiable** — `prettier`, `gofmt`/`gofumpt`, `black` or `ruff format`, `rustfmt`, `ktlint`, `spotless`.
- **Linter strict** — errors fail CI, not warnings-no-one-reads.
- **Pre-commit hook** runs both.
- **EditorConfig** for indentation, line endings, charset.
- **Single style per language per repo.** Mixing is worse than any particular choice.

## Tooling baseline per repo

| Concern | Tool |
|---|---|
| Formatter | `prettier` / `gofmt` / `black` / `rustfmt` |
| Linter | `eslint` / `golangci-lint` / `ruff` / `clippy` |
| Type checker | `tsc` / `mypy` or `pyright` |
| Complexity | `gocyclo`, `gocognit`, `eslint-plugin-sonarjs`, `radon` |
| Dead code | `knip`, `ts-prune`, `staticcheck`, `vulture` |
| Vulnerabilities | `npm audit` / `govulncheck` / `pip-audit` / `cargo audit` |
| Security | `semgrep`, `gosec`, `bandit`, `eslint-plugin-security` |
| Duplicate code | `jscpd`, `dupl` |
| Spell-check | `cspell` / `typos` |

All configured, all in CI, all failing the build on violation.

## Documentation

### Code comments

- Doc comments on exported / public API: what it does, parameters, returns, errors, examples.
- Implementation comments only for non-obvious WHY.

### Repo-level

- **README** — what, why, how to run, how to test.
- **CONTRIBUTING** — branch model, PR template, local setup.
- **ADRs** (Architecture Decision Records) — append-only record of significant decisions with context + consequences.
- **CHANGELOG** — keep a changelog (Keep-a-Changelog format is fine).
- **API reference** generated from code (TypeDoc, godoc, Sphinx, Javadoc).

## Commits & reviews

- **Conventional Commits** (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`, `perf:`, `ci:`).
- **Small PRs** — < 400 lines of diff when possible. Review quality tanks above that.
- **One logical change per PR.** Unrelated changes get their own PR.
- **Describe WHY** in the PR body, not just WHAT.
- **Every PR is reviewed**, including from senior engineers. Nobody self-merges to main except for mechanical updates with justification.
- **Review checklist**: correctness, tests, security impact, performance impact, readability, naming, no dead code.
- Link the issue/ticket; reference the spec.

## Dependencies

- **Evaluate before adding.** Maintenance, license, transitive footprint, last release date, issues/response time.
- **Pin direct dependencies.** Use a lockfile and commit it.
- **Update intentionally** (Dependabot/Renovate scheduled, not drive-by).
- **Justify every new dep** in the PR description.
- Prefer **stdlib** where possible; the best dependency is the one you did not add.
- Strip unused deps (`knip`, `depcheck`, `go mod tidy`).

## Make illegal states unrepresentable

- Discriminated unions for variants.
- Branded / newtype for IDs (`type UserId = Brand<string, "UserId">`).
- Enum types for closed sets; never a bare string.
- Parse at the boundary; downstream receives a `ValidEmail`, not a `string`.
- Builder with required fields enforced at compile time (`Partial<T>` → `Required<T>` step).

## Premature abstraction vs premature concrete

- **Don't abstract too early.** Two similar pieces of code with the same shape but different reasons for existing should stay duplicate. Abstraction couples them.
- **Don't avoid abstraction forever.** Three or four copies is a signal.
- The abstraction's interface is an opinion; it will bias all future code. Choose it deliberately.

## Performance-aware defaults

(cross-ref `performance.md`)

- Right data structure for the access pattern.
- Preallocate when size is known.
- Avoid O(n²) in hot paths.
- Don't micro-optimize without profiling.

## Security-aware defaults

(cross-ref `security.md`)

- Parameterized queries, escaping, authz checks, input validation as rules of the language.
- Principle of least privilege in API shape (no over-permissive endpoints).

## Before marking Ready (checklist)

For each PR, answer:

- [ ] Names reveal intent; no abbreviations or `d`/`tmp`/`foo`?
- [ ] Every public function has explicit, precise types? No `any`?
- [ ] Functions are small, do one thing, ≤ 3 parameters?
- [ ] No boolean-flag parameters?
- [ ] Error handling: typed, wrapped with context, no silent swallow?
- [ ] Nulls pushed to the boundary; domain uses Option / non-null types?
- [ ] Immutability by default; mutation is explicit and local?
- [ ] Domain has no imports from infra/UI?
- [ ] Dependencies injected; no hidden globals?
- [ ] Illegal states unrepresentable (newtypes, unions, enums)?
- [ ] Complexity within limits (cyclomatic ≤ 10, lines ≤ 50, nesting ≤ 3)?
- [ ] Linter and formatter pass with no suppressions (or suppressions justified in-line)?
- [ ] Type checker strict mode passes?
- [ ] No commented-out code, no TODO without a ticket?
- [ ] Comments explain WHY, not WHAT?
- [ ] Public API documented?
- [ ] PR diff < 400 lines and single-purpose?
- [ ] Conventional commit message?

If any box is "no" on relevant surface, it is a blocker, not a nit.

## Forbidden

- `any` in TypeScript without an `eslint-disable` + reason.
- `// @ts-ignore` (use `@ts-expect-error` with reason).
- `interface{}` / `any` in Go outside serialization adapters.
- `Any` in Python (mypy/pyright strict enforced).
- Raw types in Java / Kotlin.
- `unsafe` in Rust without an invariants comment.
- Catching and discarding exceptions (`catch {}`).
- `throw`-based control flow.
- Hidden mutation of parameters.
- Static singletons for I/O.
- Global mutable state outside explicit stores.
- Magic numbers and magic strings in logic.
- Commented-out code committed.
- Dead code, unused exports, unused parameters.
- `TODO` / `FIXME` / `HACK` without a linked ticket and deadline.
- Suppressing a lint rule without a recorded justification.
- Swapping a type for `any`/`unknown` to silence the type checker.
- Merging with failing formatter / linter / type checker.
- PRs mixing refactor + behavior change.
- Files > 1000 lines or functions > 100 lines without explicit exception.
- Design-by-committee abstractions with no current caller.
