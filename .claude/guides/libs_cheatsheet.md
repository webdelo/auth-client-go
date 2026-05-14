# Libs Reference

Guides are loaded **fully** by triggers. No per-rule selection needed.

### TFY — Testify (32 patterns)
Packages: `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`, `github.com/stretchr/testify/mock`, `github.com/stretchr/testify/suite`
Guide: `.claude/guides/libs/testify.md`
assert/require for test value checks, mock for interface stubs with call verification, suite for test lifecycle (setup/teardown)

### MKR — Mockery (3 patterns)
Packages: `github.com/vektra/mockery/v2`
Guide: `.claude/guides/libs/mockery.md`
CLI mock generator from Go interfaces; config via .mockery.yaml, generates mock structs with Expecter API

### GNK — Gonkey (39 patterns)
Packages: `github.com/Issengaard/gonkey_grpc`, `github.com/lamoda/gonkey`
Guide: `.claude/guides/libs/gonkey.md`
YAML-based integration testing for gRPC/REST APIs; DB fixtures (PG, CH, Redis), HTTP mocks, response verification

### UFX — Uber FX (33 patterns)
Packages: `go.uber.org/fx`
Guide: `.claude/guides/libs/uber_fx.md`
DI framework: fx.Provide/fx.Module for constructor registration, fx.As for interface binding, lifecycle hooks (OnStart/OnStop), fxm_/fxs_/fxh_ modules

### CNC — Concurrency Patterns (11 patterns)
Packages: `sync`, `golang.org/x/sync/errgroup`
Guide: `.claude/guides/libs/concurrency_patterns.md`
Ready-to-use Go concurrency patterns: worker pool, fan-out/fan-in, pipeline, semaphore, errgroup, rate limiter, circuit breaker, graceful shutdown

### SCR — Scratch CLI (32 patterns)
Packages: `scratch (CLI binary)`
Guide: `.claude/guides/libs/scratch_usage.md`
CLI code generator for Go service scaffolding from scratch.yaml: servers, config, metrics (Prometheus), health checks, DB adapters, DI wiring

### FVO — Financial Value Objects (14 patterns)
Packages: `github.com/webdelo/financial-values/assetquantity`, `github.com/webdelo/financial-values/percent`, `github.com/webdelo/financial-values/quantity`
Guide: `.claude/guides/libs/value_objects.md`
Financial arithmetic: AssetQuantity (price/volume with Asset precision), Percent (fractions), Quantity (dimensionless); rounding via TickSize/BaseSize

### CLG — Context Logger (12 patterns)
Packages: `github.com/webdelo/go-ctx-logger`
Guide: `.claude/guides/libs/ctxlogger.md`
Context-aware zap wrapper: WithField/WithFields for context enrichment, structured logging with trace/request IDs through call chain
