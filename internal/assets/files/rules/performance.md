# Rules — Performance

Performance is a feature and a correctness concern (a timeout is a bug). Any
new or changed code on a hot path, a user-facing request, a background job, or
an integration MUST be evaluated against this file before declaring Ready.

## Principle

- **Measure before optimizing.** Profile and benchmark first. "I think X is slow" is not data.
- **Measure after optimizing.** Compare before/after numbers with the same workload.
- **Optimize the dominant cost.** Amdahl's law: a 2× speedup on 5% of runtime is a rounding error.
- **Design for the p95/p99, not the average.** Users feel tail latency.
- **Prefer clear code + right data structure over cleverness.** Most perf wins come from reducing work, not micro-optimization.
- **Budget up-front.** Define latency and resource budgets per feature, then measure against them.

## Performance budgets

Before writing a new feature, declare:
- **Target p50 / p95 / p99 latency** end-to-end (user-perceived)
- **Max memory per request**
- **Max DB queries per request**
- **Max external-API calls per request**
- **Max payload size (request + response)**

Defaults (tune per product):
- Interactive API: p95 ≤ 300 ms, p99 ≤ 1 s, ≤ 10 DB queries/request, ≤ 3 external calls/request.
- Background job: defined by SLO of the downstream consumer.
- Web page: LCP ≤ 2.5 s on mid-range mobile + 4G; INP ≤ 200 ms; CLS ≤ 0.1.

Budget violations are blockers, not nits.

## Algorithmic complexity

- Know the Big-O of hot-path code. Write it in a comment when non-obvious.
- Avoid O(n²) loops over user-scalable collections.
- Prefer hash maps / sets over repeated linear scans.
- Precompute when possible; memoize pure functions; cache derived values that are expensive and stable.
- Sort once, search many.

## Database

### Queries

- **No N+1.** Use joins, batch loaders (DataLoader), `IN (...)`, or eager loading. Detect with query logs in integration tests.
- **`EXPLAIN` every new query** that hits a table > 10k rows. Verify the plan uses indexes; no seq scans on large tables.
- **No `SELECT *`** — list columns. Smaller payloads, clearer intent, index-only scans possible.
- **Parameterized queries only** (also a security rule).
- **`LIMIT` everything** returning lists. Paginate.
- **`ORDER BY` always pairs with an index or a small result set.**

### Pagination

- **Cursor-based (keyset)** for feeds, long tables, anything > 1000 rows. `WHERE id > ? ORDER BY id LIMIT n`.
- **Avoid `OFFSET`** for deep pages — it scans skipped rows.
- **Cap page size.** Hard limit server-side (e.g. 100). Reject requests above it.
- Return a stable cursor/token, not a mutable index.

### Indexes

- Every new query pattern → consider an index.
- **Composite indexes** match the query's `WHERE` + `ORDER BY` order; column order matters.
- **Covering indexes** (include columns) avoid heap lookups.
- **Partial indexes** for sparse predicates (e.g. `WHERE deleted_at IS NULL`).
- Drop unused indexes — they cost on writes.
- Monitor index bloat and fragmentation.

### Transactions & locks

- **Keep transactions short.** Nothing that waits on external I/O inside a transaction.
- **Pick the right isolation level.** Default (read committed) is usually correct; only use serializable when you actually need it (and accept the retry cost).
- **Lock in a consistent order** across code paths to avoid deadlocks.
- Prefer row-level locks (`SELECT ... FOR UPDATE`) over table-level.
- `FOR UPDATE SKIP LOCKED` for queue-like workloads.

### Connections & pooling

- **Pool connections.** Size = function of worker count × DB max connections. Never unbounded.
- Pool timeout + retries with jitter.
- **Read replicas** for read-heavy workloads; app routes reads to replica user.
- Prepared statements / statement cache enabled.

### Migrations at scale

- Large table alterations: online migration tools (pg-repack, gh-ost, pt-online-schema-change) or multi-step (add nullable → backfill → enforce not null).
- `CREATE INDEX CONCURRENTLY` on Postgres large tables.
- Never take an `ACCESS EXCLUSIVE` lock in prime time.

## Caching

- **Cache when the same computation is repeated** and the input space is bounded.
- **Layers**: browser → CDN → edge cache → app cache → Redis/Memcached → DB query cache.
- **TTL** reflects staleness tolerance; short TTL for data with fast invalidation, long TTL for immutable data.
- **Invalidation strategies**: write-through, write-behind, TTL, explicit purge on event. Pick one and document.
- **Stampede protection**: singleflight / request coalescing / probabilistic early expiration to avoid thundering herd on cache miss.
- **Cache keys** are deterministic, versioned (`v2:user:123`). Version bump ⇒ safe invalidation.
- **Negative caching** for known-absent results — but keep TTL short.
- Do not cache user-specific data in shared caches without scoping the key by user/tenant.
- **HTTP caching**: `ETag` + `If-None-Match`, `Last-Modified` + `If-Modified-Since`, `Cache-Control` with `public`/`private`/`max-age`/`s-maxage`/`stale-while-revalidate`.

## Concurrency

- **Bound concurrency.** Never spawn an unbounded number of goroutines/threads/workers per request.
- **Worker pools** sized to the downstream bottleneck (CPU, DB pool, rate limit).
- **Backpressure**: reject/queue when overloaded; do not silently drop.
- **Timeouts on every I/O** (DB, HTTP, Redis, disk). A call without a timeout is a bug.
- **Context propagation**: pass request context through to cancel work when the client disconnects.
- Avoid hot-loop locks; use sharded maps, atomic counters, or lock-free structures.
- Prefer immutable/copy-on-write data over shared mutable state.

## I/O

- **Batch**. One call for 100 items beats 100 calls for 1 item. Debounce and coalesce.
- **Stream** large payloads; do not buffer whole files in memory.
- **Compression**: gzip/brotli on responses > 1 KB (text). Enable at the edge / proxy.
- **Keep-alive / connection reuse** for outbound HTTP clients. Reuse `http.Client` / `axios` instances.
- **HTTP/2 or HTTP/3** at the edge.
- **DNS caching** for outbound services; avoid re-resolving per request.
- **TLS session resumption** enabled.

## Memory

- **Stream** JSON/CSV/XML parsers for large inputs (don't `json.Decode` a 500 MB array).
- **Discipline allocations in hot loops.** Pre-allocate slices with known capacity (`make([]T, 0, n)`).
- **Object pools** (`sync.Pool`, buffer pools) only for measured hot allocations.
- **Avoid leaks**: goroutines blocked on channels, unbounded maps/queues, listeners never removed, closures capturing large state.
- **Watch retain graphs** on long-lived caches; bounded LRU > unbounded map.

## Network & payload

- Reduce roundtrips: batching, GraphQL/BFF, HTTP/2 multiplexing.
- **Payload size**: strip nulls, use enums/codes over long strings, compress repeated fields, consider binary (protobuf, MessagePack) on hot paths.
- **Pagination + partial responses** (`fields=` query param) instead of giant blobs.
- **CDN for static assets.** Cache-busted URLs (`app.9f3a.js`) + far-future `Cache-Control`.
- Minimize the number of origins the frontend must contact (preconnect/preload when unavoidable).

## Frontend (web)

### Core Web Vitals budgets

- **LCP** ≤ 2.5 s (p75 mobile 4G)
- **INP** ≤ 200 ms
- **CLS** ≤ 0.1
- **TBT** ≤ 300 ms
- **Time to Interactive** ≤ 3.5 s

### Bundle & code

- Set a **bundle size budget** per route; CI fails if exceeded. Baseline: initial JS ≤ 170 KB gzipped.
- **Code-split by route.** Lazy-load below-the-fold features.
- Tree-shake. Audit bundle with `source-map-explorer` / `rollup-plugin-visualizer`.
- No SSR → CSR hydration mismatch; prefer server components / islands where possible.
- Avoid moment.js, lodash in full (`lodash-es` with tree shake or `lodash.get` named imports).

### Images

- Use modern formats (**AVIF / WebP**) with fallback.
- **`srcset` + `sizes`** for responsive images. Never serve a 4000px image to a 400px viewport.
- `loading="lazy"` + `decoding="async"` for below-fold images.
- `width`/`height` attributes set to avoid CLS.

### Fonts

- `font-display: swap` (or `optional` for critical text).
- Preload critical fonts.
- Subset fonts to the glyphs actually used.
- Self-host to avoid third-party handshake.

### Scripts & styles

- `async` / `defer` non-critical scripts. Critical `<script>` inline sparingly.
- Remove render-blocking CSS; critical CSS inline, rest deferred.
- Preconnect to required origins; preload high-priority assets.
- Third-party scripts (analytics, chat, A/B test) budgeted, deferred, or loaded on interaction.

### Rendering

- Avoid layout thrash: read DOM once, write once; batch with `requestAnimationFrame`.
- Virtualize long lists (`react-window`, tanstack-virtual).
- Avoid state updates that re-render large trees; memoize (`React.memo`, `useMemo`, `useCallback`) with correct dependency arrays.
- Debounce/throttle input handlers.

## Observability for performance

- **SLOs** defined per user-facing endpoint: latency target, error budget.
- Metrics: **p50, p95, p99 latency**; request rate; error rate; saturation (CPU, memory, DB pool).
- **Tracing**: OpenTelemetry end-to-end for any request crossing services. Attach DB/external spans.
- **Profiling**: `pprof` (Go), `py-spy` (Python), `clinic` (Node), async profiler (JVM) available in prod (safely, sampled).
- **Continuous profiling** tools (Pyroscope, Parca) for long-tail regressions.
- Keep **metric cardinality under control** — do not label by user ID / trace ID.
- Log hot-path errors at sampled rate; avoid log-per-request in tight loops.

## Load testing

- Every latency-sensitive feature has a load test: **k6, Locust, Gatling, or similar**.
- Test with a **representative workload** (real distribution of endpoints/inputs).
- **Baseline first, then optimize.** Keep baseline numbers checked into the repo.
- Run a **soak test** (hours) to catch leaks and long-tail effects.
- Run a **spike test** to verify autoscaling / backpressure.
- Load tests never run against production without explicit coordination.

## Benchmarks

- `go test -bench`, `pytest-benchmark`, JMH, `benchmark.js`, Criterion — use the ecosystem-native tool.
- Benchmarks live next to the code they cover.
- Allocations tracked (`b.ReportAllocs()`); bytes/op matters as much as ns/op.
- **Regression guards in CI** for hot-path benchmarks: fail if regression > N%.

## Queues & background jobs

- **Idempotent jobs** — any job may be retried.
- **Dead letter queue** after N retries with exponential backoff + jitter.
- **Visibility timeout** > max expected processing time; extend it for long jobs.
- **Throughput limits per queue** — both to protect downstream and to bill fairly.
- Priority queues for SLA-sensitive work.
- Fan-out with care — measure tail latency across partitions.
- Prefer **pull** consumers (SQS-style) over push when throughput is variable.

## Rate limits & backpressure

- External APIs you call have limits — **respect them** with token-bucket clients, retries with jitter, circuit breakers.
- When overloaded, **shed load** (reject with 429/503) instead of queueing forever.
- Circuit breakers (open / half-open / closed) on unstable dependencies.
- **Bulkhead** critical features from non-critical ones (separate pools / threads) so a slow feature does not starve the rest.

## Configuration & build

- **Release / production builds** (minified, dead-code-eliminated). Never ship dev builds.
- Compiler/runtime flags tuned: Go `GOGC` / `GOMAXPROCS`, JVM heap / GC choice, Node `--max-old-space-size`.
- **Profile-Guided Optimization** (PGO) where supported and hot paths justify it.
- Precompile regex / templates / SQL statements at startup, not per request.

## Data structures & language idioms

- Use the right container: `Map`/`Set` for lookup, arrays for sequence, ring buffers for streams.
- Preallocate known sizes. `make(map[K]V, n)` / `make([]T, 0, n)`.
- Avoid reflection in hot paths (`encoding/json` for large hot-loop payloads — consider `easyjson`, `jsoniter`, `sonic`, `simdjson`).
- String building: `strings.Builder`, `bytes.Buffer`, `StringBuilder` — never repeated `+=` in a loop on large strings.

## Logging on hot paths

- **Level check before formatting** (`if log.IsDebugEnabled()`).
- **Structured logging** — do not format strings for fields.
- **Sample** verbose logs in hot paths (1 in N).
- Do not log full request/response bodies on hot endpoints.

## Startup & cold start

- **Lazy-init** non-critical dependencies; do not block boot on optional services.
- **Pre-warm** connection pools, caches, JIT-hot paths before receiving traffic (readiness probe).
- Readiness vs liveness probes distinguish "can take traffic" from "alive".
- **Cold start on serverless**: keep package small, minimize init work, use provisioned concurrency for latency-critical paths.

## AI-specific (LLMs)

- **Stream responses** to the user as tokens arrive. Perceived latency matters more than total latency.
- **Prompt caching** for stable prefixes (system prompts, large context). Order the prompt so cached parts come first.
- **Batch** independent LLM calls when possible.
- **Token budgets per request and per user** — tokens are cost and latency.
- Timeouts and cancellation propagate through LLM calls.
- **Fallback to smaller/cheaper models** on spikes; degrade gracefully.
- Cache deterministic or low-temperature calls by prompt hash when results are reusable.

## Mobile (native / hybrid)

- **Main thread is sacred.** Never do I/O, crypto, JSON parsing, or DB work on the UI thread.
- **Cold start** ≤ 2 s; measure every release.
- Image/asset sizes budgeted; adaptive icons.
- Background work respects OS throttling (WorkManager, BGTaskScheduler).
- Battery: avoid wake locks, polling, high-frequency GPS.

## Regression detection

- **Benchmarks / load-test metrics tracked per commit** on main (baseline artifact).
- CI fails on p95 / bundle-size / memory regression beyond tolerance.
- Dashboards alert on SLO burn rate, not just instantaneous thresholds.

## Scaling

- **Stateless services scale horizontally.** Session / user state lives in a shared store.
- **Vertical scale last** — it's a dead end.
- **Autoscale on the right signal** (queue depth, p95 latency, CPU) — not just CPU.
- **Graceful degradation** strategies declared per feature (disable search ranking, serve stale cache, show last-known-good).

## Before marking Ready (checklist)

For each PR that touches a performance-sensitive surface, answer:

- [ ] Latency and resource budget declared and measured?
- [ ] `EXPLAIN` run on new queries against realistic data?
- [ ] No N+1, no `SELECT *`, no missing `LIMIT`?
- [ ] Indexes added for new query patterns?
- [ ] Timeouts on every external call?
- [ ] Bounded concurrency (no unlimited fan-out)?
- [ ] Pagination cursor-based and page size capped?
- [ ] Caching strategy (hit rate, TTL, invalidation) documented when added?
- [ ] Stampede protection where appropriate?
- [ ] Large payloads streamed, not buffered?
- [ ] Benchmarks / load test run; numbers attached to PR?
- [ ] Observability: traces, p95/p99 metrics, SLO updated?
- [ ] Log volume bounded in hot paths?
- [ ] Frontend: bundle budget respected; CWV measured on real device/emulated throttled network?
- [ ] Background jobs idempotent with DLQ + backoff?
- [ ] Regression threshold defined and enforced?

If any box is "no" and that surface exists, it is a blocker, not a nit.

## Forbidden

- Shipping perf changes **without before/after numbers**.
- Unbounded concurrency (`go func()` in a per-request loop without a bound, `Promise.all` over user-controlled arrays).
- I/O or DB calls inside a tight loop that could be batched.
- `SELECT *` on any table.
- `OFFSET` pagination on > 10k rows.
- Missing timeouts on external calls.
- Unbounded in-memory caches / maps / queues.
- Logging full request bodies on every request in prod.
- Synchronous expensive work on the UI/main thread.
- Loading full files/responses into memory when streaming is possible.
- Importing entire utility libraries when a single function is needed.
- Serving dev/debug builds in production.
- Disabling a perf budget check without a recorded justification.
