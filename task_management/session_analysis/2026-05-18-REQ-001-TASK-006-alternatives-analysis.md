# REQ-001-TASK-006 Alternatives Analysis

## metadata
- date: 2026-05-18
- task: REQ-001-TASK-006
- author: orchestrator
- topic: How to demonstrate `UnaryInterceptor` skip-list vs. auth-required behavior in `lib/examples/grpc_interceptor/main.go` without adding any new `go.sum` entries.

---

## Context

TASK-006 spec for the gRPC example:

> Тривиальный сервис `Echo` (proto inline через `reflection` пакет или предсгенерированные stubs в `examples/grpc_interceptor/proto/` — выбрать **упрощённый вариант**: использовать `grpc.NewServer(grpc.UnaryInterceptor(client.UnaryInterceptor()))` **без custom proto**, через `grpc.health.v1.Health` сервис как demo target).
> Один метод требует auth, второй (`/grpc.health.v1.Health/Check`) пропускается через дефолтный `openMethods` registry.
> Demo client отправляет 3 RPC: с валидным токеном, без токена, на health-check без токена.

DOD also requires: examples MUST NOT add new entries to `go.sum`.

Constraint: `defaultOpenMethods()` in [lib/client.go:248-253](../../lib/client.go) hardcodes both `/grpc.health.v1.Health/Check` AND `/grpc.health.v1.Health/Watch` as always-open. `WithOpenMethods(...)` can only **add** methods to the open set, never remove. So the "protected" method **cannot** be `Health/Check` or `Health/Watch`.

The question: which method to use as the "protected" demo target?

---

## Options

### Option A — Use `/grpc.health.v1.Health/List` as the protected method ✅ CHOSEN

The standard `grpc.health.v1.Health` service has **three** methods (per [google.golang.org/grpc/health/grpc_health_v1/health_grpc.pb.go:269](file:///Users/a.popov/go/pkg/mod/google.golang.org/grpc@v1.79.3/health/grpc_health_v1/health_grpc.pb.go)):

| Method | Kind | In default openMethods? |
|---|---|---|
| `Check` | unary | ✅ yes (skipped) |
| `List` | unary | ❌ no (protected) |
| `Watch` | server-stream | ✅ yes (but stream, not unary) |

`Health/List` is unary, satisfies `UnaryServerInterceptor`, and is **not** in default openMethods → it is auth-required.

The demo flow is exactly the spec:
- Client → `Health/Check` without metadata → SDK interceptor skips → returns SERVING (RPC #3 in the spec).
- Client → `Health/List` with valid Bearer → SDK validates → returns service list (RPC #1).
- Client → `Health/List` without metadata → SDK returns `codes.Unauthenticated` (RPC #2).

**Pros:**
- Zero new `go.sum` entries (`grpc/health` + `grpc/health/grpc_health_v1` are inside `google.golang.org/grpc@v1.79.3`, already pinned).
- Zero custom proto, zero `protoc` invocations, zero `grpc.ServiceDesc` hand-crafting.
- The example is a single `main.go` ~150 lines.
- `health.NewServer()` returns a ready `*health.Server` that already implements `Check`/`List`/`Watch` correctly.
- Direct evidence that the SDK's default open-methods registry works as advertised, AND that any other Health-service method is gated.

**Cons:**
- Slight reader-surprise: "why use `/List` and not a method literally named `Protected`?" — mitigated by a single-line comment in `main.go` and a sentence in the README.
- `Health/List` returns a `HealthListResponse` with a map of services rather than user data — but the example logs the gated outcome, not the payload, so this is fine.

---

### Option B — Hand-crafted `grpc.ServiceDesc` for a custom service path

Register a fake service like `/demo.v1.Demo/Whoami` via raw `grpc.ServiceDesc` + `grpc.MethodDesc`. The request/response wire types could reuse `HealthCheckRequest`/`HealthCheckResponse` from `grpc_health_v1` to avoid a new proto.

**Pros:**
- More "honest" service naming (`Demo/Whoami` reads better than `Health/List`).
- Demonstrates raw `grpc.ServiceDesc` API (educational).

**Cons:**
- ~50 extra lines of boilerplate per method: hand-written codec function, manual `dec(in)` / interceptor wiring (see [health_grpc.pb.go:219-235](file:///Users/a.popov/go/pkg/mod/google.golang.org/grpc@v1.79.3/health/grpc_health_v1/health_grpc.pb.go) for what each method handler looks like).
- The example becomes harder to read — readers would have to learn `ServiceDesc` mechanics that aren't the focus of the SDK demo.
- Reusing `HealthCheckRequest` as the wire type for a `Whoami` RPC is confusing — fields don't match semantics.

**Verdict:** rejected — violates "минимализм: каждый пример демонстрирует ровно одну фичу".

---

### Option C — Pre-generated proto stubs in `lib/examples/grpc_interceptor/proto/`

Define a tiny `demo.proto`, run `protoc` once, commit the generated `.pb.go` files.

**Pros:**
- Clean, idiomatic gRPC code.
- Closest to what a real consumer would do.

**Cons:**
- Adds `protoc` to the toolchain (developers running `make build-examples` shouldn't need `protoc`).
- Generated `.pb.go` files are large (~200 lines for one method) and check-in noise.
- The TASK-006 spec explicitly says **«выбрать упрощённый вариант: ... без custom proto»** — Option C is the rejected variant.

**Verdict:** rejected by spec.

---

### Option D — Reflection-only / unknown-service-handler tricks

Use `grpc.UnknownServiceHandler` to catch any unknown method. Have the demo client call `/anything.Anywhere/Whatever`.

**Pros:**
- No proto, no service desc.

**Cons:**
- `grpc.UnknownServiceHandler` operates at the **stream** layer — `UnaryInterceptor` does NOT fire on unknown methods. The auth check would be bypassed entirely; the demo would not show what the spec wants to show.

**Verdict:** rejected — doesn't actually trigger the unary interceptor.

---

## Decision

**Option A (Health/List)** is chosen. It satisfies the TASK-006 spec literally, requires zero new dependencies, zero custom proto, and keeps the example file small and focused on the one feature it advertises: how to attach the SDK's `UnaryInterceptor` to a real gRPC server and observe the default open-methods registry skip the health-check method while gating everything else.

The README (`lib/examples/grpc_interceptor/README.md`) will include a one-line explanation: «`Health/List` используется как защищённый метод, потому что `Check` и `Watch` входят в дефолтный `openMethods` registry SDK, а `List` — нет».

---

## Secondary decision — `build-examples` output directory

The current `Makefile` target builds to `bin/`; TASK-006 WHAT item 6 and the DOD specify `.temp/examples/<name>`. `.temp/` is already in `.gitignore`. The PRP updates the `build-examples` target accordingly and updates `clean` to drop `.temp/examples/`. No alternatives — the task spec is prescriptive.
