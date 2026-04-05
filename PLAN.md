# k8s-crondash — Plan

## Overview

Web dashboard for watching Kubernetes CronJobs, displaying their status, and manually triggering them.

**Tech stack:** Go, GoFiber v3, templ, HTMX

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   Browser                        │
│   templ-rendered HTML + HTMX interactivity       │
└──────────────┬──────────────────────────────────┘
               │ HTTP (Basic Auth)
┌──────────────▼──────────────────────────────────┐
│              GoFiber v3 (HTTP server)             │
│  Middleware:                                      │
│    Basic Auth (AUTH_USERNAME / AUTH_PASSWORD)     │
│  Routes:                                         │
│    GET  /healthz            → liveness probe      │
│    GET  /readyz             → readiness probe      │
│    GET  /                   → dashboard            │
│    GET  /cronjobs           → cronjob table (HTMX) │
│    POST /trigger/:ns/:name  → manual trigger       │
└──────────────┬──────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────┐
│           Internal packages                      │
│                                                  │
│  state/    → cached K8s state (polling goroutine  │
│              + background sync, serves all reads)  │
│                                                  │
│  k8s/      → thin wrapper around client-go       │
│              list cronjobs, create manual jobs    │
│                                                  │
│  handlers/ → fiber route handlers                │
│              (own interface for k8s service)      │
│  views/    → templ templates                     │
└─────────────────────────────────────────────────┘
```

### Why a Cache/State Layer?

Without it, every HTMX poll from every browser hits the K8s API directly — N users × 1 request per `REFRESH_INTERVAL` seconds. A background goroutine (polling on a ticker) syncs state at a configurable interval, and all HTTP handlers read from a `sync.RWMutex`-guarded cache. This:

- Eliminates thundering-herd API calls
- Makes "connection lost" detection trivial (track last successful sync timestamp)

## Authentication

- **Basic Auth (MVP):** `AUTH_USERNAME` and `AUTH_PASSWORD` (env vars or CLI flags) required. Fiber's `basicauth` middleware validates credentials using SHA-256 hashed passwords (`Users` map with `{SHA256}<base64>` format). The middleware handles comparison internally, so plaintext passwords are never stored on the `Config` struct.
- **Startup validation:** kong enforces required fields at parse time — app exits with a clear error if `AUTH_USERNAME` or `AUTH_PASSWORD` is missing.
- **Request logging:** Fiber's `logger` middleware (registered before `basicauth`) logs every request with status code, method, path, and IP — failed auth attempts appear as `401` entries.
- **K8s auth** (separate concern):
  - **Primary:** In-cluster (service account) — for production deployment
  - **Fallback:** Kubeconfig file (`~/.kube/config` or `KUBECONFIG` env var) — for local dev
  - Auto-detect: attempt in-cluster first, fall back to kubeconfig

## Configuration

All config via environment variables with sensible defaults. Ad-hoc `internal/config` package with a `Config` struct and `Validate() error` method — fails fast on invalid/missing values at startup.

| Env Var | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:3000` | HTTP listen address |
| `NAMESPACE` | (empty = all) | Namespace to watch; empty watches all |
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig (dev only) |
| `REFRESH_INTERVAL` | `5` | HTMX poll interval in seconds |
| `JOB_HISTORY_LIMIT` | `5` | Max child jobs to fetch per cronjob |
| `AUTH_USERNAME` | (required) | Basic auth username |
| `AUTH_PASSWORD` | (required) | Basic auth password |

## Package Layout

```
k8s-crondash/
  main.go              → wire everything, load config, start server
  internal/
    config/
      config.go        → Config struct (kong tags), Load(), Validate() for semantic checks
    k8s/
      client.go        → k8s client setup (in-cluster + kubeconfig fallback)
      cronjobs.go      → ListCronJobs, GetCronJob, TriggerCronJob
      jobs.go          → ListJobs (child jobs of a cronjob)
    state/
      store.go         → cached state store (sync.RWMutex + polling goroutine)
    handlers/
      dashboard.go     → GET / (render full page), GET /cronjobs (HTMX partial)
      cronjob.go       → POST /trigger/:ns/:name (returns HTMX fragment)
      interface.go     → CronJobService interface (owned by consumer)
    views/
      layout.templ     → base HTML shell (head, nav, scripts)
      dashboard.templ  → cronjob table/grid
      partials.templ   → row updates, job list panel, toasts
```

### Interface Between Handlers and K8s

Handlers own the interface (consumer-defined):

```go
// internal/handlers/interface.go
type CronJobService interface {
    ListCronJobs(ctx context.Context) ([]k8s.CronJobDisplay, error)
    TriggerCronJob(ctx context.Context, ns, name string) error
}
```

The `state.Store` implements this interface. Handler tests inject a mock. `k8s/` remains a concrete package with no handler knowledge.

### CronJobDisplay Struct

`CronJobDisplay` lives in `internal/k8s/cronjobs.go` (where it's produced). Extract to a shared package only if a second consumer needs it later.

## Logging

Structured logging via `log/slog` (stdlib since Go 1.21). All handler and K8s operations log with request-scoped fields:

- Fiber `logger` middleware logs every request with `time`, `ip`, `status`, `latency`, `method`, `path`, `error` — covers failed auth (401s)
- Handler middleware injects `namespace`, `cronjob_name`, `method`, `path` into the slog context (Phase 3+)
- K8s operations log with `component=k8s`, `operation`, duration
- Trigger operations log with `component=trigger`, `namespace`, `name`, `success`

No external logging library — `slog` is sufficient for this scope.

## Dependencies

- `github.com/gofiber/fiber/v3` — HTTP server
- `github.com/a-h/templ` — typed HTML templates
- `github.com/alecthomas/kong` — CLI flag and env var config parsing
- `k8s.io/client-go` — Kubernetes API client
- missing.css loaded via CDN (`https://unpkg.com/missing.css@1.2.0/dist/missing.min.css`, integrity `sha256-+bBeBYUh9+UNWDnPnXnxnT56osQnODQd6JzO8wZ9ZBo=`)
- HTMX loaded via CDN (no build step)
- Stdlib for logging (`log/slog`), password hashing (`crypto/sha256`, `encoding/base64`)

## Testing Strategy

- Unit tests only — never test against a real cluster
- `k8s.io/client-go` `fake.NewClientset` for all k8s package tests
- Handler tests mock the `CronJobService` interface
- templ view tests using `templ.ExecuteToString()` + substring assertions (catches rendering regressions)
- Handler integration tests using `net/http/httptest` with mocked service — validates routing and serialization
- Run with `go test ./...`

## Job Running Status

The dashboard must signal when a job is already running for a CronJob:
- For each CronJob, check if any child Job has `Active > 0` or has a `Running` condition
- Display as a status indicator in the table (e.g., badge or icon)
- Trigger button should warn if a job is already running
- This requires fetching child jobs during the cronjob list call (done by the background informer)

## MVP Scope

1. **Config** — kong-based config from env vars + CLI flags with `Config` struct, `Load()`, `Validate()` for semantic checks
2. **K8s client** — connect to cluster (in-cluster + kubeconfig fallback), list cronjobs
3. **Cached state** — background informer-backed store, serves all reads
4. **Basic auth** — fiber `basicauth` middleware with SHA-256 hashed passwords (`Users` map), probe endpoints excluded, request logging via `logger` middleware
5. **Dashboard view** — table: name, namespace, schedule, suspend status, running status, last successful run, last failure
6. **Manual trigger** — confirmation modal (HTMX swap) → creates a one-off Job from the CronJob spec, with explicit concurrency check
7. **Auto-refresh** — HTMX polling (configurable interval) reads from cache
8. **Health probes** — `/healthz` (liveness, simple 200) and `/readyz` (readiness, checks cache freshness)
9. **Graceful shutdown** — signal handling (`SIGTERM`/`SIGINT`) via `signal.NotifyContext`, fiber `Shutdown()` with timeout
10. **Structured logging** — `log/slog` throughout, Fiber `logger` middleware for request logging

## Trigger Safety

- Confirmation modal before firing a trigger (HTMX swap)
- **Concurrency check:** before creating a manual Job, list active (running) child Jobs of the CronJob (match on `ownerReferences`). If any are active, reject the trigger with a clear error. This is necessary because `concurrencyPolicy` on the CronJob object only controls the CronJob controller's behavior — it does **not** apply to manually created Jobs.
- **Single replica only:** `replicaCount` must be `1`. `ownerReferences` establishes parent-child garbage collection but does **not** make job creation idempotent. Multiple replicas receiving concurrent trigger requests would create duplicate manual jobs. If you need HA, add server-side idempotency (Post-MVP).
- **Context propagation:** `context.Context` from the Fiber request is wired through to K8s API calls. If a user disconnects mid-request, the K8s API call is cancelled via `c.Context()`.

## Health & Probes

- `GET /healthz` — returns 200, confirms the HTTP server is running (no auth required)
- `GET /readyz` — returns 200 only if the cache has been synced recently (last successful poll < 2× poll interval). Uses an atomic bool set by the background polling goroutine — no K8s API call per probe.
- Both wired as fiber routes, excluded from auth middleware
- Helm chart configures `livenessProbe` and `readinessProbe` on these endpoints

## Graceful Shutdown

- Listen for `SIGTERM` and `SIGINT` via `signal.NotifyContext`
- On signal: stop the background polling goroutine, stop accepting new connections, drain in-flight requests (fiber `Shutdown()` with timeout)
- Close k8s client connections cleanly

## Build & Dev Workflow

- `just generate` — runs `go tool templ generate` to compile `.templ` files
- `just build` — depends on `generate`, then compiles the Go binary
- `just dev` — watch for `.templ` + `.go` changes, auto-generate and rebuild
- `just vendor` — tidy + vendor dependencies
- Dockerfile build stage runs `templ generate` before `go build`

## Static Assets (CSS/JS)

- **MVP:** HTMX and missing.css loaded from CDN
- **Future:** vendor and embed via `go:embed`, serve from GoFiber static route — works in air-gapped clusters

## Packaging & Deployment

### Docker

Multi-stage `Dockerfile`:

- **Stage 1 (build):** Go alpine builder, `apk add just` and compile static binary
- **Stage 2 (runtime):** `scratch` base, copy binary only
- Built binary will be `k8s-crondash` with ldflags for version/commit/date
- Expose port 3000 (configurable via `LISTEN_ADDR`)

### Helm Chart

```
deploy/charts/k8s-crondash/
  Chart.yaml              # version: 0.1.0, appVersion: 0.1.0 (alpha/experimental)
  values.yaml
  templates/
    deployment.yaml
    service.yaml
    serviceaccount.yaml
    clusterrole.yaml        # conditional: only when rbac.namespaced=false
    clusterrolebinding.yaml # conditional: only when rbac.namespaced=false
    role.yaml               # conditional: only when rbac.namespaced=true
    rolebinding.yaml        # conditional: only when rbac.namespaced=true
    configmap.yaml          # optional: override env defaults
    secret.yaml             # AUTH_USERNAME, AUTH_PASSWORD
    ingress.yaml            # optional
    _helpers.tpl
    NOTES.txt
```

**values.yaml** key settings:

| Value | Default | Description |
|---|---|---|
| `replicaCount` | `1` | Number of pods (**must be 1** — see Trigger Safety) |
| `image.repository` | `k8s-crondash` | Container image |
| `image.tag` | chart appVersion | Image tag |
| `service.port` | `80` | Service port (targets container 3000) |
| `rbac.create` | `true` | Create RBAC resources |
| `rbac.namespaced` | `false` | Use Role/RoleBinding instead of ClusterRole/ClusterRoleBinding |
| `auth.username` | (required) | Basic auth username (stored in Secret) |
| `auth.password` | (required) | Basic auth password (stored in Secret) |
| `env.listenAddr` | `:3000` | `LISTEN_ADDR` |
| `env.namespace` | `""` | `NAMESPACE` (empty = all) |
| `env.refreshInterval` | `"5"` | `REFRESH_INTERVAL` |
| `env.jobHistoryLimit` | `"5"` | `JOB_HISTORY_LIMIT` |
| `ingress.enabled` | `false` | Enable ingress |
| `resources` | minimal requests | Pod resource limits |
| `livenessProbe` | HTTP GET `/healthz` | Liveness probe config |
| `readinessProbe` | HTTP GET `/readyz` | Readiness probe config |

**RBAC permissions needed:**
- `get`, `list`, `watch` on `cronjobs` and `jobs`
- `create` on `jobs` (for manual trigger)
- When `rbac.namespaced=true` + `NAMESPACE` is set: `Role` + `RoleBinding` scoped to that namespace — `ClusterRole` and `ClusterRoleBinding` templates are excluded entirely
- When `rbac.namespaced=false`: `ClusterRole` + `ClusterRoleBinding` — `Role` and `RoleBinding` templates are excluded entirely

**Replica count guard** in `_helpers.tpl`:
```yaml
{{- if gt (int .Values.replicaCount) 1 }}
{{- fail "replicaCount must be 1 — see Trigger Safety in README" }}
{{- end }}
```

Deployment spec uses `strategy: Recreate` to prevent rolling duplicates.

## UX Error States

| Scenario | UX Response |
|---|---|
| K8s API unreachable (sync fails) | Show last-known data with "Connection lost" banner; background goroutine retries on next sync cycle |
| Zero CronJobs in namespace | Empty state: "No CronJobs found in namespace [all]" with guidance text |
| Trigger fails (suspended, RBAC denied, API timeout, already running) | Toast notification with error message; trigger button re-enables |
| CronJob is suspended | Disable trigger button, show "Suspended" badge |
| Partial sync failure (some namespaces error) | Show last-known data, yellow warning indicator |
| Trigger succeeds | `POST /trigger/:ns/:name` returns HTMX fragment with toast + updated row — instant feedback, no poll dependency |

Note: status data comes from the cached store, which may be up to the informer's sync interval behind. The trigger POST returns an updated partial immediately so the user sees the result without waiting for the next poll.

## Multi-Namespace Display

When `NAMESPACE` is empty (watch all), the table includes a visible `namespace` column. When scoped to a single namespace, the column is hidden to reduce noise. The `REFRESH_INTERVAL` is rendered as `hx-trigger="every ${REFRESH_INTERVAL}s"` in the template, wired from config — not hardcoded.

## Post-MVP

### Security & Compliance
- Vendor and embed HTMX + missing.css via `go:embed` (air-gapped clusters)
- RBAC-aware UI: grey out trigger if user can't create jobs

### Observability
- Expandable rows showing job history
- Job logs viewer (stream via API)
- Upgrade state layer from polling to client-go informers for near-real-time updates

### UX
- Namespace selector/filter
- Dry-run mode for triggers
- Server-side idempotency for triggers (enables multi-replica HA)
- Post-trigger card update: force cache sync (`store.sync()`) after successful trigger and return updated card HTML, so the user sees the new active job immediately without waiting for the next poll cycle
- Add `GetCronJob(ctx, ns, name)` to `CronJobService` interface + store for single-CronJob lookups (avoids filtering full cache for the confirmation modal)

### Packaging
- Helm chart hardening: `helm lint`, `helm template` in CI, integration test against kind cluster
- Chart versioning and release automation

## TODO

### Phase 1 — Scaffold & Config
- [x] `internal/config/config.go` — `Config` struct with kong tags, `Load()`, `Validate()` for semantic checks (TCP address, bounds)
- [x] `main.go` — fiber server, `basicauth` middleware with SHA-256 hashed passwords (`Users` map), `logger` middleware for request logging, probe endpoints excluded, health endpoints, structured logging (`slog`), graceful shutdown
- [x] `justfile` — `generate` recipe
- [x] Checkpoint: `just run` starts server, `/healthz` returns 200, non-probe routes require auth, `--help` shows all flags

### Phase 2 — K8s Client & Cached State
- [x] Promote k8s.io/client-go to direct dep, add k8s.io/api + k8s.io/apimachinery, run just vendor
- [x] `internal/k8s/client.go` — `NewClientSet` (in-cluster + kubeconfig fallback, structured logging)
- [x] `internal/k8s/cronjobs.go` — `CronJobDisplay` struct (Name, Namespace, Schedule, Suspend, Running, LastSuccess, LastFailure, ActiveJobs) + `ListCronJobs` (enriches with child job status)
- [x] `internal/k8s/jobs.go` — `ListJobs` (child jobs via `ownerReferences`) + `isJobRunning` helper
- [x] `internal/state/store.go` — Store struct (`sync.RWMutex`, cache `[]CronJobDisplay`, `lastSync`, `ready atomic.Bool`) + `NewStore` (starts polling goroutine) + `ListCronJobs` (reads cache) + `IsReady`
- [x] Wire `main.go` — create clientset, create store, update `/readyz` to check `store.IsReady()`, context cancellation stops polling
- [x] Unit tests: `internal/k8s/cronjobs_test.go` + `jobs_test.go` using `fake.NewClientset`
- [x] Unit tests: `internal/state/store_test.go` — cache reads, ready flag, stale data behavior
- [x] Run `just fmt` + `just lint`
- [x] Checkpoint: can list cronjobs + running status from a real cluster

### Phase 3a — Templ Scaffold & Hello World
- [x] Create `internal/views/` package
- [x] `internal/views/layout.templ` — `Layout(title string)` base HTML shell: `<!DOCTYPE html>`, `<head>` with charset, viewport, title, missing.css CDN (pinned v1.2.0 with integrity hash), HTMX CDN (pinned version with integrity hash), `<body>` with app header + `{ children... }` content slot
- [x] `internal/views/index.templ` — minimal `Index()` component using `Layout("k8s-crondash")` with placeholder heading. Temporary — replaced by real dashboard template in Phase 3c.
- [x] `internal/views/render.go` — `Render(c fiber.Ctx, comp templ.Component) error` helper: sets `Content-Type: text/html`, calls `comp.Render(c.Context(), c.Response().BodyWriter())`. Needed because Fiber does not natively support templ components.
- [x] Wire `GET /` in `main.go` — replace placeholder `c.SendString("k8s-crondash dashboard")` with `views.Render(c, views.Index())`, import generated `views` package
- [x] Update `justfile` — make `build` recipe depend on `generate` (or `run` depend on both), so `.templ` files are always compiled before the binary
- [x] Run `just generate` → verify `_templ.go` files produced in `internal/views/`
- [x] Run `just vendor` → vendored deps include templ runtime (already in go.mod but vendor tree must reflect generated imports)
- [x] Run `just build` → verify full compilation
- [x] Checkpoint: `just run` → browser shows styled hello world page, missing.css applied, HTMX loaded (verify in dev tools network tab)

### Phase 3b — Handlers & Service Interface
- [x] Create `internal/handlers/` package
- [x] `internal/handlers/interface.go` — `CronJobService` interface with `ListCronJobs(ctx context.Context) ([]k8s.CronJobDisplay, error)` (only methods consumed by read-only dashboard; `TriggerCronJob` added in Phase 4)
- [x] Refactor `internal/state/store.go` — change `ListCronJobs()` signature to `ListCronJobs(ctx context.Context) ([]k8s.CronJobDisplay, error)` to satisfy `CronJobService` interface (returns cached data; ctx reserved for future use; error always nil for cache reads). Update `store_test.go` callers accordingly.
- [x] `internal/handlers/dashboard.go` — `DashboardHandler` struct (fields: `service CronJobService`, `refreshInterval int`, `showNamespace bool`) + `NewDashboardHandler(service CronJobService, refreshInterval int, showNamespace bool) *DashboardHandler`
  - `Index(c fiber.Ctx) error` — calls `service.ListCronJobs(c.Context())`, renders full page (still hello world template from 3a; real dashboard UI in Phase 3c)
  - `CronJobs(c fiber.Ctx) error` — calls `service.ListCronJobs(c.Context())`, returns simple HTML/text response showing cronjob count (proves route + data flow through interface)
- [x] Wire in `main.go` — construct `handlers.NewDashboardHandler(store, cfg.RefreshInterval, cfg.Namespace == "")`, register `app.Get("/", handler.Index)` + `app.Get("/cronjobs", handler.CronJobs)`, remove old inline `GET /` handler
- [x] Compile-time interface assertion: `var _ handlers.CronJobService = (*state.Store)(nil)` in `main.go` or `store.go`
- [x] Run `just build` + manual test: `curl -u user:pass http://localhost:3000/` returns HTML, `curl -u user:pass http://localhost:3000/cronjobs` returns data from store
- [x] Checkpoint: route structure matches architecture diagram, handlers decoupled from K8s via interface, data flows through `CronJobService` → handler → response

### Phase 3c — Dashboard Table, Partials & Auto-Refresh
- [x] `internal/views/dashboard.templ` — `Dashboard(jobs []k8s.CronJobDisplay, showNamespace bool, refreshInterval int)` full-page component using `Layout`: `<table>` with columns (name, namespace [conditional on `showNamespace`], schedule, suspended, running, last success, last failure). Container `<div>` wraps `<tbody>` with HTMX attributes: `hx-get="/cronjobs"`, `hx-trigger="every ${refreshInterval}s"`, `hx-swap="innerHTML"`
- [x] `internal/views/partials.templ` — `CronJobTableBody(jobs []k8s.CronJobDisplay, showNamespace bool)` renders `<tbody>` with one `<tr>` per CronJob (status badges for suspended/running, formatted timestamps, active job count); `EmptyState(namespace string)` renders "No CronJobs found in namespace [namespace]" message
- [x] Remove `internal/views/index.templ` — replaced by `dashboard.templ`
- [x] Update `internal/handlers/dashboard.go` — `Index` renders `views.Dashboard(jobs, showNamespace, refreshInterval)`; `CronJobs` renders `views.CronJobTableBody(jobs, showNamespace)` (or `views.EmptyState` when `len(jobs) == 0`); handle service errors (return 500)
- [x] `internal/handlers/dashboard_test.go` — handler unit tests with mock `CronJobService`: `GET /` returns 200 with table HTML, `GET /cronjobs` returns partial `<tbody>`, empty data returns empty state, service error returns 500. Use Fiber test utilities + `httptest`.
- [x] `internal/views/views_test.go` — templ view tests: `templ.ExecuteToString()` + substring assertions for `Dashboard` (table headers, namespace column conditional), `CronJobTableBody` (data cells, status badges), `EmptyState` (correct message text)
- [x] Run `just fmt` + `just lint` + `just test`
- [x] Checkpoint: open browser, see live cronjob table auto-refreshing; `curl /cronjobs` returns table body partial

### Phase 3d — Modern Card-Based Dashboard (Visual Redesign)

Replace the plain `<table>` layout with a responsive card grid using missing.css utility classes. No external CSS file — a tiny `<style>` block in `layout.templ` for the pulse animation is the only addition.

Design principles:
- **Cards over rows** — each CronJob is a `.box` card in a `.grid` layout, responsive via `data-cols` (`data-cols@s="1"` mobile, `data-cols@l="3"` desktop)
- **Visual hierarchy** — `.bold` on name, `.mono-font .secondary-font` on schedule, colorway chips (`.chip.ok`/`.warn`/`.bad`/`.plain`) for status instead of inline styles
- **Status dots** — small colored dot (`●`) before status text, pulsing animation for running jobs
- **Dark mode** — free from missing.css via `prefers-color-scheme`
- **Clean empty state** — `.box.plain .center` with friendly message

Changes:

- [x] `internal/views/layout.templ` — add `<style>` block in `<head>` with: `@keyframes pulse` animation, `.status-dot` (`.status-dot--ok`, `--warn`, `--bad`, `--running`) rules (~10 lines total)
- [x] `internal/views/dashboard.templ` — replace `<table>` with `<div class="grid spacious" data-cols@s="1" data-cols@l="3">` wrapping `CronJobCards` partial; keep HTMX poll attributes on the grid container (`hx-get="/cronjobs"`, `hx-trigger`, `hx-swap="innerHTML"`)
- [x] `internal/views/partials.templ` —
  - Add `CronJobCards(jobs []k8s.CronJobDisplay, showNamespace bool)` rendering one `.box` card per CronJob using missing.css classes: `.flex-row` for card header (name + status chip), `.mono-font` for schedule, `formatTime` for timestamps, status dot + `.chip.ok`/`.warn`/`.bad`/`.plain` badges
  - Remove `CronJobTableBody` (replaced by `CronJobCards`)
  - Keep `EmptyState` — restyle with `.box.plain .center`
- [x] Update `internal/handlers/dashboard.go` — `CronJobs` handler renders `views.CronJobCards(jobs, showNamespace)` (or `views.EmptyState` when `len(jobs) == 0`)
- [x] Update `internal/views/views_test.go` — adapt tests from table assertions to card assertions (`.box` presence, `.chip.ok`/`.warn` classes, formatted time output)
- [x] Update `internal/handlers/dashboard_test.go` — update substring assertions to match new card HTML structure instead of `<table>`/`<tr>`/`<td>`
- [x] Run `just fmt` + `just lint` + `just test`
- [x] Checkpoint: open browser, see responsive card grid with status dots, chips; resize to mobile → single column; verify dark mode via OS toggle

### Phase 4a — Trigger Backend & Interface

- [x] `internal/k8s/cronjobs.go` — add `TriggerCronJob(ctx context.Context, clientset kubernetes.Interface, ns, name string) error`: fetch CronJob (return descriptive error if not found), reject if suspended, list active child jobs via `listChildJobs` (concurrency check — reject if any running), create Job from CronJob `.spec.jobTemplate` with `ownerReferences` + `batch.kubernetes.io/cronjob` label
- [x] `internal/k8s/cronjobs_test.go` — trigger tests: success (verifies Job created with correct spec, labels, ownerReferences), already running (rejected with error), suspended (rejected with error), not found (error)
- [x] `internal/handlers/interface.go` — add `TriggerCronJob(ctx context.Context, ns, name string) error` to `CronJobService`
- [x] `internal/state/store.go` — add `TriggerCronJob(ctx context.Context, ns, name string) error` that delegates to `k8s.TriggerCronJob` (pass-through to K8s API, no cache interaction for writes)
- [x] Run `just fmt` + `just lint` + `just test`
- [x] Checkpoint: `TriggerCronJob` creates a real Job in a real cluster, concurrency/suspend guards work

### Phase 4b — Trigger UI & Handler

- [x] `internal/views/layout.templ` — add `<div id="modal-container"></div>` (after `<main>`) and `<div id="toast-container" style="position:fixed;bottom:1rem;right:1rem;z-index:100"></div>` (before `</body>`) as OOB injection targets
- [x] `internal/views/partials.templ` — add `TriggerConfirmModal(job k8s.CronJobDisplay)`: `<dialog open>` with job name, namespace, current status context, confirm button (`hx-post="/trigger/{ns}/{name}"` targeting `#modal-container`, `hx-swap="innerHTML"`), cancel button (sends request targeting `#modal-container` to clear it)
- [x] `internal/views/partials.templ` — add `Toast(message string, ok bool)`: OOB fragment (`hx-swap-oob="true"` targeting `#toast-container`), styled with `.chip.ok` or `.chip.bad`, auto-dismiss via inline `<script>setTimeout(()=>{document.getElementById('toast-container').innerHTML=''},4000)</script>`
- [x] `internal/views/partials.templ` — update `cronJobCard`: add trigger button (`<button hx-get="/trigger-confirm/{ns}/{name}" hx-target="#modal-container" hx-swap="innerHTML">`), disabled via `disabled` attribute if suspended, visual warning if running (`ActiveJobs > 0`)
- [x] `internal/handlers/cronjob.go` — `TriggerHandler` struct with `CronJobService` + `showNamespace bool`:
  - `ConfirmModal(c fiber.Ctx) error`: parse `:ns` `:name`, fetch data from store by filtering `ListCronJobs` result by ns/name, render `TriggerConfirmModal` partial
  - `Trigger(c fiber.Ctx) error`: parse `:ns` `:name`, call `service.TriggerCronJob`, on success return `Toast("Job triggered", true)` (clears modal), on error return `Toast(errMsg, false)` (clears modal)
- [x] `internal/handlers/cronjob_test.go` — trigger handler tests with mock service: confirm modal returns `<dialog>` HTML, success returns toast OOB fragment, already-running returns error toast, suspended returns error toast, service error returns 500 + error toast
- [x] `internal/views/views_test.go` — view tests: `TriggerConfirmModal` renders dialog with job name and confirm button, `Toast` renders OOB fragment with correct target and auto-dismiss script
- [x] `main.go` — construct `TriggerHandler`, register `GET /trigger-confirm/:ns/:name` → `ConfirmModal` + `POST /trigger/:ns/:name` → `Trigger`; compile-time interface assertion still passes
- [x] Run `just fmt` + `just lint` + `just test`
- [-] Checkpoint: click trigger button → modal appears → confirm → toast shows success → card updates on next poll (~5s); cancel dismisses modal; suspended card has disabled trigger button; running card shows warning on trigger button

Fix bugs in current implementation of phase 4b:
- [x] clicking "clear" to dismiss the trigger confirm modal does not work

### Phase 4c — Progressive Enhancement (No-JS Fallback)

Make all interactive features work without JavaScript. HTMX enhances the experience when JS is available, but the app is fully functional via standard HTML navigation.

**Core principle:** Every interactive element gets a semantic HTML fallback. The server detects `HX-Request` header to decide: partial (HTMX) vs full-page (no-JS).

#### Design Decisions

- **Flash params vs cookies:** Flash messages are passed as `?flash=...&flash-type=ok|bad` query parameters on redirects. Simpler than cookies (no middleware, no session store). Compromise: flash text appears in URLs and server access logs. Acceptable because the app is behind basic auth and messages are operational (e.g., "Job triggered"), not sensitive.
- **TriggerConfirmPage is a separate template** from TriggerConfirmModal. The modal uses `<dialog>` + `onclick` + HTMX attributes; the page uses `<form>` + `<a>` + no dialog. Different enough to warrant separate templates.
- **Auto-refresh without JS:** A visible "Refresh" link/button on the dashboard, not `<meta http-equiv="refresh">`. Cleaner UX, no HTML validation concerns, gives the user explicit control.

#### Implementation TODO

- [x] **`internal/views/partials.templ` — Trigger buttons: `<a>` instead of `<button>`**
   - Change trigger buttons from `<button hx-get="...">` to `<a href="/trigger-confirm/{ns}/{name}" hx-get="..." hx-target="#modal-container" hx-swap="innerHTML">`
   - Suspended jobs: keep `<button disabled>` (already correct)
   - Three instances: suspended (unchanged), running (becomes `<a>`), idle (becomes `<a>`)
   - No-JS: link navigates to full confirmation page. JS: HTMX intercepts, injects modal.

- [x] **`internal/views/partials.templ` — TriggerConfirmModal: `<form>` + `<a>` cancel**
   - Wrap confirm button in `<form method="POST" action="/trigger/{ns}/{name}" hx-post="..." hx-target="#modal-container" hx-swap="innerHTML">`
   - Cancel button → `<a href="/" onclick="event.preventDefault();this.closest('dialog').close();document.getElementById('modal-container').innerHTML=''">Cancel</a>`
   - Keep `onclick` for JS (closes modal without navigation), `href="/"` gives no-JS a real target

- [x] **`internal/views/partials.templ` — New `TriggerConfirmPage` template**
   - Full-page confirmation wrapped in `Layout("k8s-crondash")`
   - Same visual content as modal but as a regular page section (no `<dialog>`)
   - Uses `<form method="POST" action="/trigger/{ns}/{name}">` with submit button
   - Cancel is `<a href="/">Cancel</a>` — plain link home
   - No HTMX attributes needed — it's a standard HTML page

- [x] **`internal/views/dashboard.templ` — Visible refresh button**
   - Add a refresh link/button at the top of the dashboard (e.g., `<a href="/" class="button">Refresh</a>`)
   - Visible to all users; no-JS users use it for manual refresh
   - HTMX users still get auto-poll; refresh button is supplementary

- [x] **`internal/views/dashboard.templ` or `layout.templ` — Flash message banner**
   - Accept `flash` and `flash-type` params in `Dashboard` template (passed from handler)
   - Render a dismissible banner at top of `<main>`: `.chip.ok` for success, `.chip.bad` for errors
   - Only rendered when `flash` param is non-empty

- [x] **`internal/handlers/cronjob.go` — `ConfirmModal`: dual response**
   - Check `HX-Request` header via `c.Get("HX-Request")`
   - HTMX request: render `TriggerConfirmModal` fragment (dialog only, no layout) — same as current behavior
   - Normal request: render `TriggerConfirmPage` (full page with layout, `<form>`, `<a>`)
   - Both return 200

- [x] **`internal/handlers/cronjob.go` — `Trigger`: dual response**
   - Check `HX-Request` header
   - HTMX request: render `Toast` partial (OOB swap) — same as current behavior
   - Normal request: redirect to `/?flash=<message>&flash-type=ok|bad` with HTTP 303 (PRG pattern)

- [x] **`internal/handlers/dashboard.go` — `Index`: pass flash params**
   - Read `flash` and `flash-type` query params from request
   - Pass them through to `Dashboard` template for banner rendering

- [x] **`internal/views/render.go` — `IsHTMX` helper**
   - Add `IsHTMX(c fiber.Ctx) bool` that returns `c.Get("HX-Request") != ""`
   - Used by handlers to branch response format

- [x] **`internal/views/views_test.go` — Update and add tests**
   - Update `TestTriggerConfirmModal_*` tests for new `<form>` and `<a>` markup
   - Add `TestTriggerConfirmPage_*` tests (full page, form action, cancel link)
   - Add flash banner rendering tests

- [x] **`internal/handlers/cronjob_test.go` — Update and add tests**
   - Add `HX-Request` header to existing tests (should still return modal fragment)
   - Add tests for no-JS flows: confirm returns full page, trigger returns 303 redirect with flash params

- [x] **`internal/handlers/dashboard_test.go` — Add flash tests**
   - Test that `GET /?flash=...&flash-type=ok` renders flash banner

- [x] **Run `just generate && go test ./... && just fmt && just lint`**

#### Validation checklist
- [x] No-JS: trigger button navigates to full confirmation page
- [x] No-JS: confirm button submits form, redirects to dashboard with flash message
- [x] No-JS: cancel link navigates to dashboard
- [x] No-JS: refresh button reloads dashboard
- [x] JS (HTMX): trigger button opens modal (same as before)
- [x] JS (HTMX): confirm fires toast, modal dismissed (same as before)
- [x] JS (HTMX): cancel closes modal client-side (same as before)
- [x] JS (HTMX): auto-refresh still works (same as before)

### Phase 5 — Packaging & Deployment (alpha)

Chart is an unstable alpha — no stability guarantees, breaking changes at any time. Rolling release: `version: 0.0.0`, `appVersion: "0.0.0"`, `image.tag: latest` in source. Semver starts when we cut a stable release.

#### 5a — Dockerfile

- [x] Create `.dockerignore` — exclude `.git/`, `build/`, `.github/`, `*.md` (except none needed in build), `mise.toml`, `renovate.json`, `.golangci.yml`, `.crush.json`, `cj.yml`, `UNLICENSE`
- [x] Create `Dockerfile` — multi-stage build:
  - **Stage 1 (builder):** `golang:1.26-alpine` (matching go.mod toolchain), `apk add git just`, `WORKDIR /src`, `COPY go.mod go.sum ./`, `COPY vendor/ vendor/`, `COPY . .`, `ARG BUILD_VERSION=0.0.0` + `ARG BUILD_COMMIT=unknown`, run `just version="${BUILD_VERSION}" commit_sha="${BUILD_COMMIT}" build` (just overrides version/commit from host, build_date computed inside container to preserve Docker cache), `mv build/k8s-crondash-linux-* /bin/k8s-crondash`
  - **Stage 2 (runtime):** `scratch`, `COPY --from=builder /bin/k8s-crondash /bin/k8s-crondash`, `EXPOSE 3000`, `ENTRYPOINT ["k8s-crondash"]`
- [x] Add `docker-build` recipe to justfile — `docker build -t k8s-crondash:latest .` (no `just build` dependency, Dockerfile handles templ + go build internally)
- [x] Verify: `just docker-build` produces image, `docker run --rm -p 3000:3000 k8s-crondash:latest --help` prints usage

#### 5b — Helm Chart

- [ ] Create `deploy/charts/k8s-crondash/Chart.yaml` — `apiVersion: v2`, `name: k8s-crondash`, `version: 0.0.0`, `appVersion: "0.0.0"`, `description: Web dashboard for Kubernetes CronJobs`
- [ ] Create `deploy/charts/k8s-crondash/values.yaml` with defaults:
  - `replicaCount: 1`
  - `image.repository: k8s-crondash`, `image.tag: latest`, `image.pullPolicy: Always`
  - `service.port: 80`, `service.targetPort: 3000`
  - `rbac.create: true`, `rbac.namespaced: false`
  - `auth.username: ""` (required), `auth.password: ""` (required)
  - `env.listenAddr: ":3000"`, `env.namespace: ""`, `env.refreshInterval: "5"`, `env.jobHistoryLimit: "5"`
  - `ingress.enabled: false`, `ingress` scaffold (className, hosts, tls)
  - `resources:` with minimal requests (cpu: 50m, memory: 64Mi)
  - `livenessProbe`: HTTP GET `/healthz` on port 3000, `periodSeconds: 10`
  - `readinessProbe`: HTTP GET `/readyz` on port 3000, `periodSeconds: 10`, `initialDelaySeconds: 5`
- [ ] Create `deploy/charts/k8s-crondash/templates/_helpers.tpl`:
  - `k8s-crondash.name`: chart name
  - `k8s-crondash.labels`: standard Helm labels (helm.sh/chart, app.kubernetes.io/name, app.kubernetes.io/instance, app.kubernetes.io/version, app.kubernetes.io/managed-by)
  - `k8s-crondash.selectorLabels`: app.kubernetes.io/name + app.kubernetes.io/instance
  - `k8s-crondash.replicaGuard`: fail if `replicaCount > 1` with message referencing Trigger Safety
- [ ] Create `deploy/charts/k8s-crondash/templates/deployment.yaml`:
  - `strategy: Recreate`
  - Single container with image, ports (3000), env vars from ConfigMap + Secret
  - `LIVEN_ADDR` from `env.listenAddr`, `NAMESPACE` from `env.namespace`, `REFRESH_INTERVAL` from `env.refreshInterval`, `JOB_HISTORY_LIMIT` from `env.jobHistoryLimit`
  - `AUTH_USERNAME` from secret key `username`, `AUTH_PASSWORD` from secret key `password`
  - `livenessProbe` + `readinessProbe` wired from values
  - Resource limits from values
  - ServiceAccount name
- [ ] Create `deploy/charts/k8s-crondash/templates/service.yaml` — ClusterIP, port mapping (servicePort → targetPort 3000)
- [ ] Create `deploy/charts/k8s-crondash/templates/serviceaccount.yaml` — always created, `automountServiceAccountToken: true`
- [ ] Create `deploy/charts/k8s-crondash/templates/secret.yaml` — `auth.username` and `auth.password` from values, `type: Opaque`
- [ ] Create `deploy/charts/k8s-crondash/templates/configmap.yaml` — `LISTEN_ADDR`, `NAMESPACE`, `REFRESH_INTERVAL`, `JOB_HISTORY_LIMIT` from values
- [ ] Create `deploy/charts/k8s-crondash/templates/clusterrole.yaml` + `clusterrolebinding.yaml` — conditional on `rbac.create` and `not rbac.namespaced`; permissions: `get`, `list`, `watch` on `cronjobs` and `jobs`, `create` on `jobs`
- [ ] Create `deploy/charts/k8s-crondash/templates/role.yaml` + `rolebinding.yaml` — conditional on `rbac.create` and `rbac.namespaced`; same permissions, scoped to `NAMESPACE` (or `"default"` if empty)
- [ ] Create `deploy/charts/k8s-crondash/templates/ingress.yaml` — conditional on `ingress.enabled`, supports `spec.ingressClassName`, host/path rules, TLS
- [ ] Create `deploy/charts/k8s-crondash/templates/NOTES.txt` — post-install message showing how to access the dashboard (port-forward command, service URL)
- [ ] Add `helm-lint` recipe to justfile — `helm lint deploy/charts/k8s-crondash`
- [ ] Add `helm-template` recipe to justfile — `helm template k8s-crondash deploy/charts/k8s-crondash`
- [ ] Verify: `just helm-lint` passes with no errors or warnings, `just helm-template` renders all templates

#### 5c — README

- [ ] Write `README.md`:
  - Project description — what k8s-crondash does (one-paragraph summary)
  - Quick start (Helm): `helm install` example with `--set auth.username=admin,auth.password=changeme`, port-forward command
  - Docker usage: `docker run` example with required env vars (`AUTH_USERNAME`, `AUTH_PASSWORD`)
  - Configuration table: all env vars (from PLAN.md Config section) with their Helm values equivalents, defaults, and descriptions
  - Development section: `just build`, `just run`, `just test`, `just lint`, `just docker-build`, `just helm-lint`

#### 5d — Manual Smoke Test

- [ ] `just docker-build` → load image into kind cluster (`kind load docker-image k8s-crondash:latest`)
- [ ] `helm install k8s-crondash deploy/charts/k8s-crondash --set auth.username=admin,auth.password=changeme` → verify pods start, probes pass
- [ ] Port-forward to service → access dashboard in browser → auth works → cronjob list renders → trigger works

> **Note:** Helm chart is considered alpha/experimental. Hardening (automated `helm lint`/`helm template` in CI, integration tests against kind) is Post-MVP.
