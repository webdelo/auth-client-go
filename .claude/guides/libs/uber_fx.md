---
prefix: UFX
name: Uber FX
packages:
  - go.uber.org/fx
triggers:
  - uber.fx
  - fx.New
  - fx.Provide
  - fx.Module
  - fx.As
  - fx.Invoke
  - fx.
  - dependency injection
  - DI
  - fxm_
  - fxs_
  - fxh_
description: "DI framework: fx.Provide/fx.Module for constructor registration, fx.As for interface binding, lifecycle hooks (OnStart/OnStop), fxm_/fxs_/fxh_ modules"
---

# Dependency Injection with Uber Fx

## UFX-001: Overview

Uber Fx is a dependency injection framework for Go that eliminates global state, ensures code reuse, and manages application lifecycle.

**Key benefits:**
- Automatic dependency graph construction
- Lifecycle management (OnStart/OnStop)
- Modular architecture
- Graph integrity validation at initialization stage
- Battle-tested (used in production at Uber)

**Documentation:**
- Official documentation: https://uber-go.github.io/fx/
- GitHub: https://github.com/uber-go/fx
- Habr article (usage practices): https://habr.com/ru/companies/kaspersky/articles/699994/

---

## UFX-002: Infrastructure/Fx Folder Structure

The project should use an organized structure for fx modules:

```
infrastructure/fx/
├── adapters/              # Adapters for data transformation between layers
│   └── fxa_*/             # fxa_ - adapter prefix
│
├── constants/             # Constants for dependency grouping
│   └── fxc_groups/        # Group tags constants
│       └── group_constants.go
│
├── hooks/                 # fx.Lifecycle hooks (OnStart/OnStop)
│   ├── fxh_grpc/          # gRPC hooks
│   │   ├── fxh_grpc_server/
│   │   └── fxh_grpc_clients/
│   ├── fxh_kafka/         # Kafka hooks
│   ├── fxh_services/      # Service lifecycle hooks
│   └── fxh_web/           # Web server hooks
│
├── in/                    # fx.In structures (dependency injection)
│   └── fxi_*/             # fxi_ - input prefix
│
├── out/                   # fx.Out structures (dependency provision)
│   └── fxo_*/             # fxo_ - output prefix
│
├── shows/                 # Provide functions (constructors)
│   ├── fxs_configs/       # Configuration providers
│   ├── fxs_logger/        # Logger providers
│   ├── fxs_metrics/       # Metrics providers
│   ├── fxs_services/      # Service providers
│   └── fxs_repositories/  # Repository providers
│
└── supplies/              # fx.Supply data (predefined values)
    └── fxsup_*/           # fxsup_ - supply prefix
```

**Naming Conventions:**
| Prefix | Purpose | Example |
|--------|---------|---------|
| `fxs_*` | Provide functions (shows) | `fxs_configs/`, `fxs_services/` |
| `fxh_*` | Lifecycle hooks | `fxh_grpc/`, `fxh_kafka/` |
| `fxi_*` | fx.In structures (input) | `fxi_grpc/`, `fxi_clients/` |
| `fxo_*` | fx.Out structures (output) | `fxo_services/`, `fxo_bybit/` |
| `fxa_*` | Adapters | `fxa_esb/`, `fxa_binance/` |
| `fxc_*` | Constants | `fxc_groups/` |
| `fxsup_*` | Supply data | `fxsup_market_data/` |

---

## UFX-003: Core Concepts

### UFX-004: fx.Provide — Constructor Registration

Registers constructor functions that create dependencies:

```go
fx.Provide(
    NewLogger,      // func() *log.Logger
    NewHandler,     // func(*log.Logger) (http.Handler, app_error)
    NewMux,         // func(fx.Lifecycle, *log.Logger) *http.ServeMux
)
```

**Features:**
- Constructors are executed lazily (only if dependency is needed)
- Results are cached (singleton by default)
- Can return error as last value
- Can return multiple values

### UFX-005: fx.Invoke — Function Execution

Functions are executed immediately during application initialization:

```go
fx.Invoke(func(mux *http.ServeMux, h http.Handler) {
    mux.Handle("/", h)
})
```

**Best practice:** Use named functions instead of lambdas:

```go
// GOOD - named functions
func CreateApp() fx.Option {
    return fx.Options(
        fx.Invoke(
            preloadCache,
            printGraph,
            registerRunners,
        ),
    )
}

func preloadCache(cc Cacher) error {
    return cc.PreloadCache()
}

// BAD - anonymous functions complicate debugging
fx.Invoke(func(cc Cacher) error {
    return cc.PreloadCache()
})
```

### UFX-006: fx.Supply — Providing Ready Values

For providing already-created values:

```go
var logger *log.Logger = log.New(os.Stdout, "", 0)
fx.Supply(logger)  // Equivalent to fx.Provide(func() *log.Logger { return logger })
```

### UFX-007: fx.Module — Named Option Groups

Organizing functionality into modules:

```go
var LoggingModule = fx.Module("logging",
    fx.Provide(NewLogger),
    fx.Invoke(SetupLogging),
)

fx.New(LoggingModule, OtherModule)
```

### UFX-008: fx.Options — Option Grouping

Combining options without module semantics:

```go
var loggingAndMetrics = fx.Options(
    LoggingModule,
    MetricsModule,
)
```

---

## UFX-009: Lifecycle API

### UFX-010: fx.Lifecycle and Hooks

Registering callbacks for startup and shutdown:

```go
type Hook struct {
    OnStart func(context.Context) error
    OnStop  func(context.Context) error
}
```

**Simple hook example:**

```go
func GRPCServerHook(s *grpc.Server, lc fx.Lifecycle) error {
    lc.Append(fx.Hook{
        OnStart: s.Start,
        OnStop:  s.Stop,
    })
    return nil
}
```

**Hook example with goroutine and channels:**

```go
func InitMarketGates(
    gateRegistry GateRegistry,
    connectors GateToMarketConnectors,
    lc fx.Lifecycle,
) error {
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            resultChan := initGates(gateRegistry, connectors.ApiHandlers)
            select {
            case <-ctx.Done():
                return ctx.Err()
            case err := <-resultChan:
                return err
            }
        },
        OnStop: func(ctx context.Context) error {
            resultChan := stopGates(gateRegistry)
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-resultChan:
                return nil
            }
        },
    })
    return nil
}
```

**IMPORTANT:** Context in OnStart is only valid during startup. For background goroutines, create separate contexts and manage them in OnStop.

---

## UFX-011: Parameter and Result Objects

### UFX-012: fx.In — Dependency Injection

Structure for declaring input dependencies:

```go
type HandlerParams struct {
    fx.In

    Logger   *log.Logger
    Database *sql.DB
    Cache    *redis.Client `optional:"true"`  // Optional dependency
}

func NewHandler(p HandlerParams) *Handler {
    if p.Cache == nil {
        // Work without cache
    }
    return &Handler{...}
}
```

### UFX-013: fx.Out — Dependency Provision

Structure for declaring output dependencies:

```go
type FxOutServices struct {
    fx.Out

    UserService  *UserService
    OrderService *OrderService `name:"main_order_service"`
}

func NewServices(db *sql.DB) FxOutServices {
    return FxOutServices{
        UserService:  NewUserService(db),
        OrderService: NewOrderService(db),
    }
}
```

---

## UFX-014: Named Dependencies

### UFX-015: Using name Tag

For distinguishing multiple instances of the same type:

```go
// Providing named dependencies
type ConnectionResult struct {
    fx.Out

    ReadWrite *sql.DB `name:"rw"`
    ReadOnly  *sql.DB `name:"ro"`
}

// Receiving named dependencies
type GatewayParams struct {
    fx.In

    WriteConn *sql.DB `name:"rw"`
    ReadConn  *sql.DB `name:"ro"`
}
```

---

## UFX-016: Value Groups

### UFX-017: Defining Group Constants

```go
// infrastructure/fx/constants/fxc_groups/group_constants.go
package fxc_groups

const (
    GroupRunners   = `group:"runners"`
    GRPCServices   = `group:"grpc_services"`
    GRPCClients    = `group:"grpc_clients"`
    KafkaConsumers = `group:"kafka_consumers"`
)
```

### UFX-018: Adding to a Group

```go
// Via fx.Out
type HandlerResult struct {
    fx.Out

    Handler Handler `group:"server"`
}

// Via fx.Annotate
fx.Annotate(
    NewUserService,
    fx.As(new(starter.Runner)),
    fx.ResultTags(fxc_groups.GroupRunners),
)
```

### UFX-019: Receiving a Group

```go
type ServerParams struct {
    fx.In

    Handlers []Handler `group:"server"`
}

func NewServer(p ServerParams) *Server {
    server := newServer()
    for _, h := range p.Handlers {
        server.Register(h)
    }
    return server
}
```

---

## UFX-020: Annotations API

### UFX-021: fx.Annotate

Adding tags without creating separate structures:

```go
fx.Provide(
    fx.Annotate(
        NewGateway,
        fx.ParamTags(`name:"ro" optional:"true"`, `name:"rw"`),
        fx.ResultTags(`name:"main_gateway"`),
    ),
)
```

### UFX-022: fx.As — Type Transformation

Registering as an interface:

```go
fx.Annotate(
    NewUserService,
    fx.As(new(UserRepository)),      // First interface
    fx.As(new(starter.Runner)),      // Second interface
)
```

### UFX-023: Multiple Interfaces from Single Component

```go
fx.Annotate(
    func(c *grpc_client.Client) *grpc_client.Client { return c },
    fx.As(new(binance_futures.AssetRegistry)),
    fx.As(new(binance_spot.AssetRegistry)),
    fx.As(new(balance_service.AssetRegistry)),
)
```

---

## UFX-024: Complete main.go Structure

```go
package main

import (
    "go.uber.org/fx"

    "myapp/infrastructure/fx/constants/fxc_groups"
    "myapp/infrastructure/fx/hooks/fxh_grpc"
    "myapp/infrastructure/fx/hooks/fxh_services"
    "myapp/infrastructure/fx/shows/fxs_configs"
    "myapp/infrastructure/fx/shows/fxs_services"
    "myapp/infrastructure/fx/supplies/fxsup_data"
)

const (
    FxStartTimeout = 60 * time.Second
    FxStopTimeout  = 15 * time.Second
)

func main() {
    fx.New(
        CreateApp(),
        fx.StartTimeout(FxStartTimeout),
        fx.StopTimeout(FxStopTimeout),
    ).Run()
}

func CreateApp() fx.Option {
    return fx.Options(
        // 1. Supply — predefined values
        fx.Supply(
            fxsup_data.DefaultSettings(),
        ),

        // 2. Provide — register all constructors
        fx.Provide(
            // Configurations
            fxs_configs.AppConfig,
            fxs_configs.DatabaseConfig,

            // Logger
            fxs_logger.NewLogger,

            // Services with annotations
            fx.Annotate(
                fxs_services.UserService,
                fx.As(new(starter.Runner)),
                fx.ResultTags(fxc_groups.GroupRunners),
            ),

            // gRPC services
            fx.Annotate(
                fxs_grpc.UserServer,
                fx.As(new(grpc.Service)),
                fx.ResultTags(fxc_groups.GRPCServices),
            ),
        ),

        // 3. Invoke — initialization with lifecycle hooks
        fx.Invoke(
            fxh_grpc.GRPCServerHook,
            fxh_services.StarterServiceHook,
        ),
    )
}
```

---

## UFX-025: Best Practices

### UFX-026: Avoid Lambdas in Production Code

```go
// GOOD
func preloadCache(cc Cacher) error { return cc.PreloadCache() }
fx.Invoke(preloadCache)

// BAD
fx.Invoke(func(cc Cacher) error { return cc.PreloadCache() })
```

### UFX-027: Don't Use Literal Types

```go
// GOOD
type AppConfig struct { Value string }
fx.Provide(func() AppConfig { return AppConfig{Value: "test"} })

// BAD
fx.Provide(func() string { return "test" })  // Conflicts with multiple string instances
```

### UFX-028: Write Validation Tests

```go
func TestValidateApp(t *testing.T) {
    err := fx.ValidateApp(CreateApp())
    require.NoError(t, err)
}
```

### UFX-029: Don't Over-Fragment Initialization

Functions in fx.Invoke should have logical meaning as separate "stages" of startup.

### UFX-030: Solution for Duplicate Component Creation

```go
// WRONG - creates two instances:
fx.Provide(
    NewRepo,
    fx.Annotate(NewRepo, fx.As(new(UserAccessor))),
)

// RIGHT - single instance:
fx.Provide(
    NewRepo,
    fx.Annotate(
        func(r *Repo) *Repo { return r },
        fx.As(new(UserAccessor)),
    ),
)
```

---

## UFX-031: Initialization Flow Diagram

```
main()
  └─> fx.New(CreateApp())
      └─> CreateApp()
          ├─> fx.Supply()          [Predefined values]
          │
          ├─> fx.Provide()         [Constructor registration]
          │   ├─> Configuration providers
          │   ├─> Service providers
          │   ├─> Repository providers
          │   └─> fx.Annotate for interfaces and groups
          │
          └─> fx.Invoke()          [Lifecycle hooks]
              ├─> Hook1 → OnStart: ... / OnStop: ...
              ├─> Hook2 → OnStart: ... / OnStop: ...
              └─> HookN → OnStart: ... / OnStop: ...

Lifecycle:
1. DI (dependency graph construction)
2. OnStart calls (in dependency order)
3. Application running
4. OnStop calls (in reverse order)
```

---

## UFX-032: Module Organization Recommendations

1. **Keep fx modules focused** — one responsibility per module
2. **Use fx for critical infrastructure** — database, cache, messaging
3. **Don't use fx for simple utilities** — not everything needs injection
4. **Group related dependencies** — use Value Groups
5. **Document group constants** — in fxc_groups/ with comments

---

## UFX-033: Reference Projects

- **funding-bot** (`/home/gremlin/go/src/github.com/webdelo/funding-bot`) — production example of uber.fx usage with complete `infrastructure/fx/` structure
