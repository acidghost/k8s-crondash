# k8s-crondash

Web dashboard for watching Kubernetes CronJobs, displaying their status, and manually triggering them. A single Go binary with a card-based UI that auto-refreshes via HTMX — no JavaScript required for core functionality.

**Status:** Alpha / experimental. No stability guarantees; breaking changes at any time.

## Quick Start (Helm)

```bash
helm install k8s-crondash deploy/charts/k8s-crondash \
  --set auth.username=admin \
  --set auth.password=changeme
```

Then access the dashboard:

```bash
export POD_NAME=$(kubectl get pods -l "app.kubernetes.io/instance=k8s-crondash" -o jsonpath="{.items[0].metadata.name}")
kubectl port-forward "$POD_NAME" 3000:3000
```

Open <http://localhost:3000> and log in with the credentials you set.

### Watch all namespaces

```bash
helm install k8s-crondash deploy/charts/k8s-crondash \
  --set auth.username=admin \
  --set auth.password=changeme \
  --set env.namespace=""
```

## Configuration

All settings are configurable via CLI flags, environment variables, or Helm values.

| Env Var | Flag | Default | Helm Value | Description |
|---|---|---|---|---|
| `CRONDASH_AUTH_USERNAME` | `--auth-username` | (required) | `auth.username` | Basic auth username |
| `CRONDASH_AUTH_PASSWORD` | `--auth-password` | (required) | `auth.password` | Basic auth password |
| `CRONDASH_LISTEN_ADDR` | `--listen-addr` | `:3000` | `env.listenAddr` | HTTP listen address |
| `CRONDASH_NAMESPACE` | `--namespace` | (empty = all) | `env.namespace` | Namespace to watch; empty watches all |
| `CRONDASH_REFRESH_INTERVAL` | `--refresh-interval` | `5` | `env.refreshInterval` | Auto-refresh interval in seconds |
| `CRONDASH_JOB_HISTORY_LIMIT` | `--job-history-limit` | `5` | `env.jobHistoryLimit` | Max child jobs to fetch per CronJob |

### Namespace scoping (Helm)

The `env.namespace` value controls both RBAC scope and the `NAMESPACE` env var:

| `env.namespace` | Effect | RBAC |
|---|---|---|
| `"-"` (default) | Watch release namespace | `Role` + `RoleBinding` |
| `""` (empty) | Watch all namespaces | `ClusterRole` + `ClusterRoleBinding` |
| `"my-ns"` | Watch a specific namespace | `Role` + `RoleBinding` scoped to `my-ns` |

## Development

Requires [Go 1.26+](https://go.dev/dl/) and a running Kubernetes cluster (or kubeconfig pointing to one). Use [mise](https://mise.jdx.dev) to install dev tools (`just`, `golangci-lint`, `helm`, `kubeconform`) as pinned in `mise.toml`:

```shell
mise install
```

```bash
just build          # compile binary (includes templ generation)
just run            # build and run
just test           # run tests
just fmt            # format Go + templ files
just lint           # run golangci-lint
just vendor         # tidy + vendor dependencies
just build-image    # build Docker image
just helm-lint      # validate Helm chart
just helm-conform   # render chart + validate with kubeconform
```

### Trigger safety

`replicaCount` must be `1`. The Helm chart enforces this at render time. Multiple replicas receiving concurrent trigger requests would create duplicate Jobs because `ownerReferences` establishes garbage collection but not idempotency.

## License

See [UNLICENSE](./UNLICENSE).
