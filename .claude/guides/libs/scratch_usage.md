---
prefix: SCR
name: Scratch CLI
packages:
  - scratch (CLI binary)
triggers:
  - scratch
  - scratch.yaml
  - scratch generate
  - prometheus
  - metrics
  - метрики
  - logging
  - логирование
  - health check
  - healthcheck
  - sentry
  - system server
  - middleware
  - инициализация проекта
  - project init
description: "CLI code generator for Go service scaffolding from scratch.yaml: servers, config, metrics (Prometheus), health checks, DB adapters, DI wiring"
---

# Scratch CLI Usage Guide

## SCR-010: Overview

**Scratch** — CLI code generator for Go backend service scaffolding.
- **Binary**: `scratch`
- **Config file**: `scratch.yaml` (project root)
- **Generated output**: `generated/scratch/` directory
- **Purpose**: generates project structure, config, infrastructure adapters, DI wiring, Docker files from `scratch.yaml`

## SCR-011: Commands

### SCR-012: `scratch new`

Create a new project with scratch scaffolding.

```bash
scratch new my-service
scratch new my-service --type api        # Project type: api (default), worker, gateway
scratch new my-service --module github.com/org/my-service  # Custom Go module path
scratch new my-service --force           # Overwrite existing directory
```

### SCR-013: `scratch generate` — PRIMARY COMMAND

Regenerate all code from `scratch.yaml`. **Run after every scratch.yaml change.**

```bash
scratch generate                  # Standard generation
scratch generate --dry-run        # Preview changes without writing files
scratch generate --verbose        # Detailed output for debugging
scratch generate --force          # Force regeneration (ignore checksums)
scratch generate --skip-docker    # Skip Docker/docker-compose generation
scratch generate --config path    # Custom config file path
```

### SCR-014: `scratch update`

Update the scratch binary to the latest version.

```bash
scratch update                    # Update to latest
scratch update --check            # Check for updates without installing
scratch update --version v1.2.3   # Update to specific version
```

### SCR-015: `scratch update-mem-bank`

Update Claude agent infrastructure (`.claude/`, `.memory_bank/` templates) from reference repository.

```bash
scratch update-mem-bank                                    # Default update
scratch update-mem-bank --repository user/repo             # Custom template repo
scratch update-mem-bank --skip-claude                      # Skip .claude/ directory update
scratch update-mem-bank --verbose                          # Detailed output
scratch update-mem-bank --dry-run                          # Preview changes
scratch update-mem-bank --force                            # Force overwrite
```

### SCR-016: `scratch update-tools`

Update project tooling configuration (linters, Makefile targets, CI configs).

```bash
scratch update-tools              # Update tools
scratch update-tools --verbose    # Detailed output
scratch update-tools --dry-run    # Preview changes
scratch update-tools --force      # Force overwrite
```

### SCR-017: `scratch init-mem-bank`

Initialize memory bank in a project that doesn't have one yet.

```bash
scratch init-mem-bank                                      # Initialize with defaults
scratch init-mem-bank --repository user/repo               # Custom template repo
scratch init-mem-bank --skip-claude                        # Skip .claude/ directory
scratch init-mem-bank --verbose                            # Detailed output
```

### SCR-018: `scratch version`

Print current scratch version.

```bash
scratch version
```

### `scratch completion`

Generate shell completion scripts.

```bash
scratch completion bash > /etc/bash_completion.d/scratch
scratch completion zsh > "${fpath[1]}/_scratch"
scratch completion fish > ~/.config/fish/completions/scratch.fish
```

## SCR-019: scratch.yaml Structure

```yaml
app:
  slug: "my-service"              # REQUIRED — project identifier (kebab-case)
  project_type: "api"             # api | worker | gateway
  env_prefix: "MY_SERVICE"        # Environment variable prefix

  storages:
    postgres:                     # PostgreSQL connection
      instances:
        - name: "main"
          database: "my_service"
    clickhouse:                   # ClickHouse analytics
      instances:
        - name: "analytics"
    aerospike:                    # Aerospike cache/storage
      instances:
        - name: "cache"
    redis:                        # Redis (use struct form, NOT `redis: true`)
      instances:
        - name: "cache"
    valkey:                       # Valkey (use struct form, NOT `valkey: true`)
      instances:
        - name: "session"

  grpc:                           # gRPC server configuration
    enabled: true
    services:
      - name: "UserService"

  kafka_settings:                 # Kafka consumers/producers
    consumers:
      - topic: "events"
        group: "my-service"
    producers:
      - topic: "notifications"

  docker:                         # Docker configuration
    base_image: "golang:1.25-alpine"
    expose_port: 8080

logger:                           # Logger configuration
  level: "info"
  format: "json"

ogen:                             # OpenAPI code generation
  spec_path: "api/openapi.yaml"
  package: "api"
```

## SCR-020: Generated Infrastructure Components

Scratch generates base service infrastructure from `scratch.yaml`. All generated files are in `generated/scratch/` and are **FORBIDDEN to edit manually**. Marker: `// Code generated by scratch, DO NOT EDIT.`

| Component | File | Purpose |
|-----------|------|---------|
| Facade | `scratch.go` | Central `Scratch` struct: Config, Server, Logger, Postgres, Prometheus Registry, zap.Logger |
| System Server | `system/system.go` | HTTP server on port `:53000` (configurable via `SYSTEM_ADDR`) |
| Configuration | `system/config.go` | Env var parsing via `caarlos0/env/v11` |
| Health checks | `system/healthcheck.go` | Pluggable health/readiness checkers |
| Logger | `logger/logger.go` | zap.Logger + Sentry + HTTP middleware with trace_id |
| gRPC Server | `grpc/server.go` | gRPC server with keepalive policies |
| PostgreSQL | `infra/storage/postgres/postgres.go` | pgxpool connection pool |
| HTTP Server | `server/server.go` | HTTP server with ogen + Prometheus metrics registry |

## SCR-021: System Server

Automatically provides:

| Endpoint | Purpose |
|----------|---------|
| `/metrics` | Prometheus metrics (PedanticRegistry) |
| `/healthcheck`, `/liveness` | Liveness probes |
| `/readycheck`, `/readiness` | Readiness probes |
| `/debug/pprof/*` | Profiling (goroutine, heap, block, trace) |

Auto-registered collectors: ProcessCollector, GoCollector, BuildInfoCollector.
OpenTelemetry: OTEL MeterProvider with Prometheus exporter, TextMapPropagator (TraceContext + Baggage).

## SCR-022: Configuration from Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `LOG_INDEX` | — | Log level (debug, info, warn, error) |
| `APP_ADDR` | `:8080` | HTTP server address |
| `GRPC_ADDR` | `:50051` | gRPC server address |
| `SYSTEM_ADDR` | `:53000` | System server address (metrics, health, pprof) |
| `ENVIRONMENT` | `development` | Mode (development/production) |
| `SENTRY_DSN` | — | Sentry DSN (optional) |
| `POSTGRES_USER` | — | **required** |
| `POSTGRES_PASSWORD` | — | **required** |
| `POSTGRES_HOST` | — | **required** |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_DATABASE` | — | **required** |
| `POSTGRES_MAX_CONNS` | `20` | Max pool connections |
| `POSTGRES_MIN_CONNS` | `2` | Min pool connections |
| `POSTGRES_MIGRATE_ENABLED` | `false` | Auto-migrate on startup |

## SCR-023: Logger

- Development mode (`ENVIRONMENT=development`): human-readable, colors, stacktrace
- Production mode (`ENVIRONMENT=production`): JSON, optimized
- Level: configured via `LOG_INDEX`
- Sentry: auto-sends errors (level >= Error) when `SENTRY_DSN` is set
- HTTP middleware: auto-enriches logs with `trace_id`, `uri`, `host`, `http_method`, `remote_addr`, `caller_service_id`
- Context: `FromContext(ctx)` / `ToContext(ctx, logger)` for logger propagation

## SCR-024: Health Checks

Built-in checkers:
- `DBChecker(name, pinger)` — PostgreSQL via `Ping()`
- `HTTPChecker(name, url)` — HTTP endpoint (GET, expects 200)
- `GRPCChecker(name, addr)` — gRPC via `grpc.health.v1`
- `CheckerFunc` — interface for custom checkers

## SCR-025: PostgreSQL

- `pgxpool.Pool` configured from env vars (`POSTGRES_*`)
- Automatic DSN construction
- Configurable connection pool (min/max conns, lifetime, idle timeout)
- Health check period for connection monitoring

## SCR-026: FX Integration Patterns

Scratch components integrate via uber/fx DI:
```
internal/infrastructure/fx/
├── modules/fxm_system/module.go    — Config, Logger, SystemServer (singleton)
├── hooks/fxh_postgres/hook.go      — PostgreSQL lifecycle
├── hooks/fxh_grpc/hook.go          — gRPC server lifecycle
├── hooks/fxh_http/hook.go          — HTTP server lifecycle
└── shows/fxs_*.go                  — Service providers
```

## SCR-027: Forbidden Duplications (Antipatterns)

Scratch already provides these — DO NOT duplicate:

| Feature | Where in scratch | How to use |
|---------|-----------------|------------|
| `/metrics` endpoint | `system/system.go` → `registerPrometheus()` | Metrics auto-available on `:53000/metrics` |
| ProcessCollector, GoCollector | `system/system.go` → `New()` | Auto-registered on PedanticRegistry |
| Health/readiness endpoints | `system/system.go` → `registerHealthchecks()` | Use `AddHealthChecker()` / `AddReadyChecker()` |
| zap.Logger init | `logger/logger.go` → `New()` | Get via DI; DO NOT create a new logger |
| Sentry integration | `logger/logger.go` → `New()` | Configure via `SENTRY_DSN` env var |
| HTTP middleware with trace_id | `logger/logger.go` → `NewHTTPMiddleware()` | Applied automatically via fx hooks |
| PostgreSQL connection pool | `infra/storage/postgres/postgres.go` | Get `*pgxpool.Pool` via DI |
| Config from env vars | `system/config.go` → `NewFromEnv()` | Get `*system.Config` via DI |

Common mistakes:
```go
// WRONG: manual metrics endpoint
http.Handle("/metrics", promhttp.Handler())

// WRONG: registering on global registry via promauto
var counter = promauto.NewCounter(...)

// WRONG: manually registering standard collectors
prometheus.MustRegister(prometheus.NewGoCollector())

// WRONG: creating new logger instead of DI
logger := ctxlogger.New(ctxlogger.Config{Level: "info"})

// WRONG: manual zap configuration
zapConfig := zap.NewProductionConfig()
logger, _ := zapConfig.Build()
```

## SCR-028: Extension Points

### SCR-029: Custom Metrics

Register on scratch's `prometheus.Registry` (get via DI from `system.System.Registry`):

```go
func NewOrderMetrics(registry *prometheus.Registry) *OrderMetrics {
    m := &OrderMetrics{
        OrdersCreated: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "crm_orders_created_total",
                Help: "Total number of orders created",
            },
            []string{"status"},
        ),
    }
    registry.MustRegister(m.OrdersCreated)
    return m
}
```

### SCR-030: ctx-logger over scratch zap.Logger

ctx-logger (`github.com/webdelo/go-ctx-logger`) wraps scratch's `zap.Logger` for context-based logging:

```go
func NewCtxLogger(scratchLogger *logger.Logger) ctxlogger.Logger {
    return ctxlogger.NewFromZap(scratchLogger.Logger)
}
```

Scratch manages base settings (level, encoding, Sentry). ctx-logger adds:
- `WithField("service", "orders")` — service-specific fields
- `FromContext(ctx)` / `ToContext(ctx)` — context propagation
- Sampling for high-volume debug logs (optional)

### SCR-031: Service-specific Health Checkers

```go
func RegisterHealthChecks(sys *system.System, db *pgxpool.Pool) {
    sys.AddHealthChecker(system.DBChecker("postgres", db))
    sys.AddReadyChecker(system.DBChecker("postgres", db))
    sys.AddReadyChecker(system.GRPCChecker("guard", "localhost:50051"))
}
```

### SCR-032: Domain Configuration

Service-specific configuration is added ON TOP of scratch config, not replacing it:

```go
type OrdersConfig struct {
    MaxItemsPerOrder  int           `env:"ORDERS_MAX_ITEMS" envDefault:"100"`
    ProcessingTimeout time.Duration `env:"ORDERS_PROCESSING_TIMEOUT" envDefault:"30s"`
}
```

## SCR-0001: After scratch.yaml change — ALWAYS run `scratch generate`

Any modification to `scratch.yaml` MUST be followed by `scratch generate`.
Use `scratch generate --dry-run` first to preview changes if unsure.

## SCR-0002: NEVER manually edit files in `generated/scratch/`

All files in `generated/scratch/` are managed by scratch. Manual edits will be
overwritten on next `scratch generate`. If customization is needed — use the
extension points documented in the generated code.

## SCR-0003: If scratch.yaml exists — read it to understand infrastructure

When working in a project with `scratch.yaml`, ALWAYS read it first to understand:
- What storages are configured (postgres, redis, clickhouse, etc.)
- Whether gRPC is enabled
- What Kafka topics are used
- Docker configuration

## SCR-0004: Use `scratch generate --verbose` for debugging

If generation fails or produces unexpected results, run with `--verbose` flag
to see detailed processing information.

## SCR-0005: Use `scratch generate --force` only for corrupted checksums

The `--force` flag bypasses checksum validation. Use only when checksums are
corrupted or manually edited files need to be overwritten.

## SCR-0006: Use `scratch generate --dry-run` for preview

Before running generation, especially after significant config changes,
use `--dry-run` to preview what files will be created/modified.

## SCR-035: Known Gotchas

### SCR-036: Valkey has no `UnmarshalYAML`

Cannot use shorthand `valkey: true` — always use struct form:
```yaml
# WRONG
valkey: true

# CORRECT
valkey:
  instances:
    - name: "session"
```

### SCR-037: Redis `redis: true` sets Instances=nil

Using `redis: true` shorthand sets `Instances=nil`, but the orchestrator checks
`len(Instances) > 0` — so no code gets generated. Always use struct form:
```yaml
# WRONG — generates nothing
redis: true

# CORRECT
redis:
  instances:
    - name: "cache"
```
