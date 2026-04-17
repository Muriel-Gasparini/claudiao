# Rules — Security

Security is a first-class quality gate, not an afterthought. Any new or changed
code that touches user input, auth, data, or external systems MUST be evaluated
against this file before declaring Ready.

## Principle

- **Deny by default.** Allow lists over block lists. Explicit authorization
  checks at every sensitive boundary. Absence of a check is a defect.
- **Trust no input.** Validate shape, type, and range at the boundary; never
  trust data from a client, a queue, a file, or a third-party API.
- **Depth in layers.** Input validation does not replace parameterization,
  escaping, or authorization. Use all three.

## Secrets

- **Never** commit secrets: API keys, tokens, passwords, private keys, signing
  keys, database URLs with credentials, OAuth client secrets.
- **Never** log secrets (including auth headers, tokens in URLs, session IDs,
  passwords even when "hashed for demo").
- **Never** send secrets to third parties (error trackers, analytics, prompt
  inputs to LLMs, debug dumps).
- Read secrets from environment variables, a secrets manager, or vault. Not
  from config files committed to the repo.
- If a secret leaks, **rotate it first**, remove from history second. Assume
  compromise.
- Add pattern-based checks: pre-commit hooks (gitleaks, trufflehog),
  `.gitignore` for `.env`, `*.pem`, `credentials.*`.

## Input validation

Validate at every **trust boundary**:
- HTTP handlers (body, query, headers, path params)
- Message consumers (Kafka, SQS, webhooks)
- File upload contents and MIME types
- CLI args and env vars that affect behavior
- Data read from external APIs

For each field check:
- Type and shape (use a schema: zod, pydantic, JSON schema, struct tags)
- Length bounds (min and max)
- Allowed character set (regex or explicit enum)
- Semantic range (dates in plausible window, IDs in owned scope)
- Encoding (reject invalid UTF-8, reject null bytes)

Reject early with a 4xx. Do not "sanitize" silently — clients must know their
input was wrong.

## Authorization

- **Authentication** proves who the caller is. **Authorization** proves they
  may do this specific thing to this specific resource.
- Check authorization **at every endpoint**, including GET endpoints. Do not
  rely on UI hiding.
- Scope every query to the caller's tenant/user/organization. IDOR
  (`GET /users/123`) must verify ownership, not just existence.
- Admin operations require explicit role/permission checks, not "logged in".
- Expiring tokens: short lifetimes, refresh flow, revocation list when needed.

## Injection

- **SQL**: always parameterized queries or a query builder. Never string
  concatenation with user input. Never `ORDER BY $userInput` without
  allow-listing the column set.
- **Command execution**: avoid `exec`, `system`, shell out in general. When
  unavoidable, never pass user input as shell args. Use the language's
  argument-array form, not a single string.
- **Template/HTML**: auto-escape by default. Only use `raw` / `html-safe` /
  `dangerouslySetInnerHTML` on content you generated yourself or sanitized
  with a vetted library (DOMPurify, bleach).
- **NoSQL**: reject operators from user input (`$gt`, `$ne` in Mongo).
  Validate that fields receive primitives, not objects.
- **LDAP, XPath, regex, prototype pollution**: same principle — parameterize
  or escape per the target's rules.

## SSRF

When code fetches a URL provided by the user or derived from user data:
- Resolve the hostname and reject private ranges (10/8, 172.16/12, 192.168/16,
  127/8, ::1, link-local, metadata IPs like 169.254.169.254).
- Force the scheme to `https://` (or an allow list of schemes).
- Cap response size and timeout aggressively.
- Disable redirects, or validate the target of each redirect with the same rules.
- Consider a dedicated outbound proxy for user-driven fetches.

## Path traversal

- `..`, absolute paths, symlinks out of the allowed root — all must be
  rejected when building paths from user input.
- Use `filepath.Clean` / `path.Clean` and then verify the result is a
  descendant of the intended base directory (compare canonical absolute paths).
- Never concatenate user input into file paths without validation.

## Deserialization

- Reject unknown fields by default when deserializing configs or API bodies.
- Never deserialize pickled/serialized objects from untrusted sources (Python
  `pickle`, PHP `unserialize`, Java serialization, YAML `!!python/object`).
- For YAML, use safe loaders only (`yaml.safe_load`).

## File uploads

- Validate MIME type **and** magic bytes, not just extension.
- Strip or reject archives that decompress beyond a limit (zip-bomb defense).
- Store uploads outside the web root; serve via a handler that enforces
  authz and content-disposition.
- For images, re-encode through a trusted library (stripping EXIF and
  preventing polyglot files).

## Crypto

- **Do not roll your own.** Use vetted libraries: libsodium, BoringSSL,
  OpenSSL via a wrapper, Web Crypto, Go's `crypto` packages.
- Passwords: Argon2id (preferred) or bcrypt with cost 12+. Never MD5, SHA-1,
  or unsalted SHA-256 for passwords.
- Random: cryptographic RNG only (`crypto/rand`, `secrets.token_bytes`,
  `window.crypto.getRandomValues`). Never `Math.random` or `rand` for tokens,
  session IDs, password resets.
- Symmetric encryption: AES-GCM or ChaCha20-Poly1305 (authenticated).
- TLS: enforce TLS 1.2+, prefer TLS 1.3. Reject weak ciphers.

## HTTP hardening

Response headers on HTML endpoints:
- `Strict-Transport-Security: max-age=31536000; includeSubDomains`
- `Content-Security-Policy` with an explicit policy (no `unsafe-inline` in
  production)
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `X-Frame-Options: DENY` (or CSP `frame-ancestors`)

## CORS

- Do not reflect `Origin` blindly. Allow list.
- Never return `Access-Control-Allow-Origin: *` with
  `Access-Control-Allow-Credentials: true`.
- For cookies: `SameSite=Lax` or `Strict` where possible; `Secure`; `HttpOnly`
  when the client does not need to read them.

## Rate limiting & abuse

- Rate limit authentication endpoints, password resets, email sends, webhook
  replays, and any expensive query.
- Keyed by user and by IP (fall back to IP when unauthenticated).
- Consider CAPTCHA / proof-of-work on abuse-prone flows.
- Per-endpoint quotas for expensive operations (image resize, PDF gen, search).

## Logging

- Log **what happened, who did it, when, and the outcome**. Do not log
  secrets, passwords, session tokens, credit card numbers, or full request
  bodies on auth routes.
- Redact PII in logs by default; opt-in for detailed logs in dev only.
- Log failed authorization attempts — they are the primary signal of attack.
- Include a correlation ID so a single request can be traced.

## Error handling

- Server errors return a generic message to the client (`"internal error"` +
  a trace ID). Never return a stack trace, a SQL error, or an internal path
  to the client.
- Log the full error server-side with context.
- 4xx errors may describe the problem ("invalid email format"), but never
  reveal whether a user exists ("user not found" vs "wrong password" leaks
  account enumeration — use a single message).

## Dependencies

- Use a lockfile (`package-lock.json`, `yarn.lock`, `go.sum`,
  `poetry.lock`, `Cargo.lock`). Commit it.
- Run a vulnerability scanner in CI (`npm audit`, `pip-audit`, `govulncheck`,
  `cargo audit`, Dependabot/Renovate).
- Pin direct dependencies; update intentionally, not drive-by.
- Be suspicious of packages with few maintainers, recent ownership changes,
  or typo-squat-adjacent names.

## AI-specific

When the product calls LLMs on behalf of users:
- Never include unredacted PII or secrets in the prompt.
- Treat LLM output as untrusted input. Do not execute it as code, SQL, or
  shell without the same validations applied to user input.
- Prompt injection is real. If the model reads from user-controlled content
  (documents, emails, web pages), treat that content as adversarial and
  sandbox any resulting actions.

## Before marking Ready

For each PR that touches a sensitive surface, answer:

- [ ] Any new user input goes through a validator?
- [ ] Authorization checked for every new endpoint / action?
- [ ] Any new SQL/shell/template use is parameterized or escaped?
- [ ] Any outbound fetch from user input has SSRF protection?
- [ ] Any file path built from user input is confined to a safe root?
- [ ] Any secret added to env/vault (not committed) and not logged?
- [ ] Error responses do not leak internals?
- [ ] Dependency additions scanned for CVEs?

If any box is "no" and that surface exists, it is a blocker, not a nit.

## Forbidden

- Disabling TLS verification in code (`verify=False`, `InsecureSkipVerify`)
  outside of clearly scoped local development.
- Hardcoded secrets anywhere, even "temporary".
- `eval`, `Function("…")`, `exec` on strings built from user input.
- Writing custom cryptography (rolling your own KDF, MAC, or cipher).
- Catching a security-relevant error and continuing silently.
- Disabling a lint/scanner rule without a recorded justification.
