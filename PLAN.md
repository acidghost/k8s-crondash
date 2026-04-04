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
- [ ] Create `internal/views/` package
- [ ] `internal/views/layout.templ` — `Layout(title string)` base HTML shell: `<!DOCTYPE html>`, `<head>` with charset, viewport, title, missing.css CDN (pinned v1.2.0 with integrity hash), HTMX CDN (pinned version with integrity hash), `<body>` with app header + `{ children... }` content slot
- [ ] `internal/views/index.templ` — minimal `Index()` component using `Layout("k8s-crondash")` with placeholder heading. Temporary — replaced by real dashboard template in Phase 3c.
- [ ] `internal/views/render.go` — `Render(c fiber.Ctx, comp templ.Component) error` helper: sets `Content-Type: text/html`, calls `comp.Render(c.Context(), c.Response().BodyWriter())`. Needed because Fiber does not natively support templ components.
- [ ] Wire `GET /` in `main.go` — replace placeholder `c.SendString("k8s-crondash dashboard")` with `views.Render(c, views.Index())`, import generated `views` package
- [ ] Update `justfile` — make `build` recipe depend on `generate` (or `run` depend on both), so `.templ` files are always compiled before the binary
- [ ] Run `just generate` → verify `_templ.go` files produced in `internal/views/`
- [ ] Run `just vendor` → vendored deps include templ runtime (already in go.mod but vendor tree must reflect generated imports)
- [ ] Run `just build` → verify full compilation
- [ ] Checkpoint: `just run` → browser shows styled hello world page, missing.css applied, HTMX loaded (verify in dev tools network tab)

### Phase 3b — Handlers & Service Interface
- [ ] Create `internal/handlers/` package
- [ ] `internal/handlers/interface.go` — `CronJobService` interface with `ListCronJobs(ctx context.Context) ([]k8s.CronJobDisplay, error)` (only methods consumed by read-only dashboard; `TriggerCronJob` added in Phase 4)
- [ ] Refactor `internal/state/store.go` — change `ListCronJobs()` signature to `ListCronJobs(ctx context.Context) ([]k8s.CronJobDisplay, error)` to satisfy `CronJobService` interface (returns cached data; ctx reserved for future use; error always nil for cache reads). Update `store_test.go` callers accordingly.
- [ ] `internal/handlers/dashboard.go` — `DashboardHandler` struct (fields: `service CronJobService`, `refreshInterval int`, `showNamespace bool`) + `NewDashboardHandler(service CronJobService, refreshInterval int, showNamespace bool) *DashboardHandler`
  - `Index(c fiber.Ctx) error` — calls `service.ListCronJobs(c.Context())`, renders full page (still hello world template from 3a; real dashboard UI in Phase 3c)
  - `CronJobs(c fiber.Ctx) error` — calls `service.ListCronJobs(c.Context())`, returns simple HTML/text response showing cronjob count (proves route + data flow through interface)
- [ ] Wire in `main.go` — construct `handlers.NewDashboardHandler(store, cfg.RefreshInterval, cfg.Namespace == "")`, register `app.Get("/", handler.Index)` + `app.Get("/cronjobs", handler.CronJobs)`, remove old inline `GET /` handler
- [ ] Compile-time interface assertion: `var _ handlers.CronJobService = (*state.Store)(nil)` in `main.go` or `store.go`
- [ ] Run `just build` + manual test: `curl -u user:pass http://localhost:3000/` returns HTML, `curl -u user:pass http://localhost:3000/cronjobs` returns data from store
- [ ] Checkpoint: route structure matches architecture diagram, handlers decoupled from K8s via interface, data flows through `CronJobService` → handler → response

### Phase 3c — Dashboard Table, Partials & Auto-Refresh
- [ ] `internal/views/dashboard.templ` — `Dashboard(jobs []k8s.CronJobDisplay, showNamespace bool, refreshInterval int)` full-page component using `Layout`: `<table>` with columns (name, namespace [conditional on `showNamespace`], schedule, suspended, running, last success, last failure). Container `<div>` wraps `<tbody>` with HTMX attributes: `hx-get="/cronjobs"`, `hx-trigger="every ${refreshInterval}s"`, `hx-swap="innerHTML"`
- [ ] `internal/views/partials.templ` — `CronJobTableBody(jobs []k8s.CronJobDisplay, showNamespace bool)` renders `<tbody>` with one `<tr>` per CronJob (status badges for suspended/running, formatted timestamps, active job count); `EmptyState(namespace string)` renders "No CronJobs found in namespace [namespace]" message
- [ ] Remove `internal/views/index.templ` — replaced by `dashboard.templ`
- [ ] Update `internal/handlers/dashboard.go` — `Index` renders `views.Dashboard(jobs, showNamespace, refreshInterval)`; `CronJobs` renders `views.CronJobTableBody(jobs, showNamespace)` (or `views.EmptyState` when `len(jobs) == 0`); handle service errors (return 500)
- [ ] `internal/handlers/dashboard_test.go` — handler unit tests with mock `CronJobService`: `GET /` returns 200 with table HTML, `GET /cronjobs` returns partial `<tbody>`, empty data returns empty state, service error returns 500. Use Fiber test utilities + `httptest`.
- [ ] `internal/views/views_test.go` — templ view tests: `templ.ExecuteToString()` + substring assertions for `Dashboard` (table headers, namespace column conditional), `CronJobTableBody` (data cells, status badges), `EmptyState` (correct message text)
- [ ] Run `just fmt` + `just lint` + `just test`
- [ ] Checkpoint: open browser, see live cronjob table auto-refreshing; `curl /cronjobs` returns table body partial

### Phase 4 — Manual Trigger
- [ ] `internal/k8s/cronjobs.go` — `TriggerCronJob` (create Job from CronJob spec, explicit concurrency check via active child Jobs)
- [ ] `internal/views/partials.templ` — confirmation modal + toast/feedback
- [ ] `internal/handlers/cronjob.go` — `POST /trigger/:ns/:name` returns HTMX fragment (toast + updated row) — instant feedback
- [ ] Disable/warn trigger button when job already running
- [ ] Wire `context.Context` from Fiber request through to K8s API calls
- [ ] Unit tests for trigger logic + handler
- [ ] Checkpoint: click trigger → confirm → job appears in cluster

### Phase 5 — Packaging & Deployment (alpha)
- [ ] `Dockerfile` — multi-stage build with `templ generate`
- [ ] Helm chart `deploy/charts/k8s-crondash/` (all templates, RBAC, probes, auth Secret, replica guard) — `version: 0.1.0`, `appVersion: 0.1.0`
- [ ] Update `README.md` with usage instructions
- [ ] Manual smoke test: `helm install` on kind cluster → dashboard accessible → trigger works

> **Note:** Helm chart is considered alpha/experimental. Hardening (automated `helm lint`/`helm template` in CI, integration tests against kind) is Post-MVP.
