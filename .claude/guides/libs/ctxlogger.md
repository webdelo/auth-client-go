---
prefix: CLG
name: Context Logger
packages:
  - github.com/webdelo/go-ctx-logger
triggers:
  - ctxlogger
  - ctx-logger
  - context logging
  - log context
  - WithField
  - context.Background
  - trace id
  - request_id
description: "Context-aware zap wrapper: WithField/WithFields for context enrichment, structured logging with trace/request IDs through call chain"
---

# Context Logger (go-ctx-logger)

## CLG-001: Overview

`github.com/webdelo/go-ctx-logger` — context-aware zap wrapper. Every log call carries context fields accumulated through the call chain.

## CLG-002: Import

```go
import ctxlogger "github.com/webdelo/go-ctx-logger"
```

## CLG-003: Core API

### CLG-004: Context Enrichment

```go
// Add fields to context — all downstream log calls will include them.
ctx = ctxlogger.WithField(ctx, zap.String("request_id", reqID))
ctx = ctxlogger.WithField(ctx, zap.String("symbol", sym.Name))
ctx = ctxlogger.WithField(ctx, zap.String("file", fileName))
```

### CLG-005: Logging

```go
log.Info(ctx, "operation started", zap.Int("batch_size", len(batch)))
log.Warn(ctx, "skipping invalid row", zap.Error(err))
log.Error(ctx, "operation failed", zap.Error(err))
log.Debug(ctx, "debug info", zap.Any("data", data))
```

## CLG-006: Rules

### CLG-007: All methods that log MUST accept `context.Context` as first parameter

```go
// GOOD
func (s *Service) Process(ctx context.Context, data Data) error {
    s.log.Info(ctx, "processing", zap.String("id", data.ID))
    return nil
}

// BAD — context.Background() loses all accumulated fields
func (s *Service) Process(data Data) error {
    s.log.Info(context.Background(), "processing", zap.String("id", data.ID))
    return nil
}
```

### CLG-008: Never use `context.Background()` in methods that can receive ctx from caller

`context.Background()` is allowed ONLY in:
- `main()` or CLI command entry points
- Test code
- Top-level goroutine spawners that create a new trace

Everywhere else — accept and propagate `ctx` from the caller.

### CLG-009: Enrich context at meaningful boundaries

Add context fields at each layer transition:

```go
// Entry point (HTTP middleware / gRPC interceptor / CLI command)
ctx = ctxlogger.WithField(ctx, zap.String("request_id", uuid.New().String()))

// Use case layer
ctx = ctxlogger.WithField(ctx, zap.String("symbol", cfg.Symbol))
ctx = ctxlogger.WithField(ctx, zap.String("operation", "load_trades"))

// Infrastructure layer
ctx = ctxlogger.WithField(ctx, zap.String("file", file.Name))
```

### CLG-010: Error wrapping — identifying context + logging fields

Two mechanisms. **Default: вариант 2 (WrapError)**. Вариант 1 — только когда контекст недоступен.

#### 1. zap fields + static `fmt.Errorf` — when context is NOT available

When the error originates where `ctx` with enriched fields is not available (e.g., pure domain logic, validation), use static error wrapping and zap fields for logging:

```go
// GOOD — static error string + zap fields for dynamic values
func (p *Parser) ParseRow(row []string) (Trade, error) {
    price, err := strconv.ParseFloat(row[1], 64)
    if err != nil {
        return Trade{}, fmt.Errorf("parse price: %w", err)
    }
    return Trade{Price: price}, nil
}

// Caller logs with zap fields:
trade, err := p.ParseRow(row)
if err != nil {
    log.Error(ctx, "parse row failed", zap.Int("row_num", i), zap.Error(err))
    return err
}

// BAD — dynamic values interpolated into error string
return fmt.Errorf("process file %s: %w", file.Name, err)
// BAD — generic message, hard to trace
return fmt.Errorf("processing failed: %w", err)
```

Always use `%w` to preserve the error chain for `errors.Is()`/`errors.As()`.

#### 2. `ctxlogger.WrapError` — default approach when context IS available

When error crosses a layer boundary (infrastructure → use case → handler), accumulated context fields (`request_id`, `symbol`, `file`, etc.) are lost because each layer has its own `ctx`. `WrapError` captures them into the error itself.

**This is the preferred approach** — use it whenever `ctx` with enriched fields is available.

**Sender** (lower layer — wraps error with context fields):

```go
func (r *Repo) GetTrades(ctx context.Context, symbol string) ([]Trade, error) {
    ctx = ctxlogger.WithField(ctx, zap.String("table", "raw_trades"))
    ctx = ctxlogger.WithField(ctx, zap.String("symbol", symbol))
    rows, err := r.conn.Query(ctx, query)
    if err != nil {
        // WrapError captures all ctx fields (request_id, symbol, table, ...)
        // fmt.Errorf adds static human-readable context to error chain
        return nil, ctxlogger.WrapError(ctx, fmt.Errorf("query trades: %w", err))
    }
    // ...
}
```

**Receiver** (upper layer — restores fields before logging):

```go
func (uc *UseCase) LoadTrades(ctx context.Context, cfg Config) error {
    trades, err := uc.repo.GetTrades(ctx, cfg.Symbol)
    if err != nil {
        // UnwrapErrorCtx restores fields (request_id, symbol, table) from error into ctx
        ctx = ctxlogger.UnwrapErrorCtx(ctx, err)
        uc.log.Error(ctx, "load trades failed", zap.Error(err))
        return err
    }
    // ...
}
```

#### When to use which

| Mechanism | When | What it preserves |
|-----------|------|-------------------|
| `ctxlogger.WrapError(ctx, err)` | **Default** — ctx with enriched fields is available | Structured logging fields (`zap.Field`) |
| zap fields + static `fmt.Errorf` | ctx is NOT available (pure logic, validation) | Human-readable error chain + structured fields at log site |

**Rule of thumb:** If the layer enriched `ctx` with `WithField` and returns an error — wrap with `WrapError`. If you receive an error and plan to log it — call `UnwrapErrorCtx` first. Dynamic values (`file.Name`, `symbol`, `id`) always go into zap fields, never into `fmt.Errorf` format strings.

#### Manual field extraction (rare)

```go
var errCtx *ctxlogger.ErrorWithLogCtx
if errors.As(err, &errCtx) {
    fields := errCtx.Fields() // []zapcore.Field
}
```

## CLG-011: Enrichment Points

| Layer | Fields to Add |
|-------|---------------|
| gRPC interceptor | `request_id` (from `x-request-id` header or generated UUID) |
| HTTP middleware | `request_id`, `method`, `path` |
| CLI command entry | `trace_id` (generated UUID), `command` |
| Use case | `operation`, domain-specific IDs (`symbol`, `strategy_id`) |
| Infrastructure | `file`, `table`, `batch_id` |

## CLG-012: Anti-Patterns

```go
// BAD: Logging without context — loses all enriched fields
func (p *Parser) parse(record []string) error {
    p.log.Warn(context.Background(), "invalid row")
    return fmt.Errorf("parse error")
}

// BAD: Storing context in struct field (violates EC-0051)
type Service struct {
    ctx context.Context  // NEVER do this
}

// BAD: Accepting logger but not context
func NewParser(log ctxlogger.Logger) *Parser  // missing ctx propagation path

// BAD: Logging error from lower layer without restoring its context fields
func (uc *UseCase) Run(ctx context.Context) error {
    err := uc.repo.Process(ctx)
    uc.log.Error(ctx, "failed", zap.Error(err)) // loses repo's fields (table, batch_id, ...)
    return err
}
// GOOD: Restore context before logging
func (uc *UseCase) Run(ctx context.Context) error {
    err := uc.repo.Process(ctx)
    ctx = ctxlogger.UnwrapErrorCtx(ctx, err)
    uc.log.Error(ctx, "failed", zap.Error(err)) // includes all downstream fields
    return err
}
```
