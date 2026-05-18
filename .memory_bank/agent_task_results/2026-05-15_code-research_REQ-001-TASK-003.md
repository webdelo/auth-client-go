# Code Research Report — REQ-001 TASK-003: ValidateAccessToken + TTL Cache + HTTPMiddleware + UnaryInterceptor

**Date:** 2026-05-18
**Researcher:** code-researcher (claude-sonnet-4-6)
**Task:** REQ-001-TASK-003

---

## 1. Where I Looked

| Repo / Path | What I searched for |
|-------------|---------------------|
| `task_management/tasks/REQ-001-TASK-003.md` | TASK spec, DOD, trigger rules |
| `task_management/plans/REQ-001-ADR.md` §A2/A4/A10/A11 + Implementation Blueprint | Pseudocode for all 5 deliverables |
| `task_management/plans/REQ-001-TASK-002-PRP.md` | What TASK-002 delivers (Cache interface with `any`, bgCtx field, openMethods) |
| `.memory_bank/agent_task_results/2026-05-15_code-research_REQ-001-TASK-002.md` | Established patterns from prior research |
| `.temp/rules/REQ-001-TASK-003/coding_scope.md` + `full_rules.md` | EC-0010/0012/0034/0056, ES-0051/0120/0140 full text |
| `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/handlers/grpc/signal/interceptors.go` | Recovery interceptor, metadata extraction |
| `/Users/a.popov/Development/TRADING/bots/auth/internal/service/httpauthmw/middleware.go` | `extractBearer`, `WWW-Authenticate`, 401 pattern |
| `/Users/a.popov/Development/TRADING/bots/auth/internal/service/tokenmanager/generate_refresh_token.go` | `sha256.Sum256` + `hex.EncodeToString` pattern |
| `/Users/a.popov/Development/TRADING/bots/auth/internal/handlers/grpc/auth/validate_token.go` | What `error_reason` strings the server actually emits |
| `/Users/a.popov/Development/TRADING/bots/auth/internal/usecase/auth/dto.go` | `ValidateTokenOutput.ErrorReason` documented values |
| `/Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service.pb.go` (lines 464–574) | Exact Go field names on `ValidateTokenRequest`/`ValidateTokenResponse` |
| `/Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service_grpc.pb.go` (lines 26/56) | `AuthServiceClient.ValidateToken` signature + `FullMethodName` constant |
| `/Users/a.popov/Development/TRADING/bots/invest/domain/structure/primitives/kline.volume.go` | `sync.Map.Range` + `.Delete` co-usage (invest) |
| `/Users/a.popov/Development/TRADING/bots/invest/pkg/generic/queue_with_pool/queue_test.go` | Benchmark file convention (`func BenchmarkX(b *testing.B)`) |
| `/Users/a.popov/Development/GOLANG/fundingbot/pkg/generic/observable/observable_benchmark_test.go` | Benchmark file convention, `b.ReportAllocs()` |

---

## 2. Findings by Topic

### 2.1 sync.Map cache with lazy-expire

No TTL-cache on `sync.Map` exists in sibling repos. The closest is `kline.volume.go` in invest
(`/Users/a.popov/Development/TRADING/bots/invest/domain/structure/primitives/kline.volume.go:96-135`),
which shows `sync.Map.Range` + `.Store`/`.Load` basic usage but no TTL.

The ADR §A2 + Implementation Blueprint is the authoritative design — write fresh.
The Blueprint pseudocode (ADR lines 1241–1298) is complete and correct. Key implementation notes:

- `sync.Map.Range` is safe to call while concurrent `Delete` calls happen — Go spec guarantees
  this (Range calls are snapshot-like; deletes within Range callback are also safe per sync.Map docs).
- `cleanupInterval = cfg.ValidateCacheTTL / 2` (set in `New` when creating default MemoryCache).
- Guard: `if cleanupInterval <= 0 { return }` — prevents zero-interval ticker panic when
  `ValidateCacheTTL == 0` (cache disabled case). Without the guard, `time.NewTicker(0)` panics.

### 2.2 Panic-recovery gRPC interceptor pattern (signal-analyzer)

**Exact file:** `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/handlers/grpc/signal/interceptors.go:99-119`

```go
func NewRecoveryInterceptor(log ctxlogger.Logger) grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (resp interface{}, err error) {   // ← named return
        defer func() {
            if r := recover(); r != nil {
                stack := debug.Stack()
                log.Error(ctx, "grpc panic recovered", ...)
                err = grpcstatus.Error(codes.Internal, "internal app_error")
            }
        }()
        return handler(ctx, req)
    }
}
```

Key: named return `(resp interface{}, err error)` — the `defer` sets `err` directly because it
captures the named return variable. Without named return, assigning to `err` inside `defer` has
no effect on the caller's return value.

For the SDK interceptor, combine this pattern with the auth-check logic; do NOT chain two separate
interceptors. The ADR §A11 makes recovery part of `UnaryInterceptor` itself (step 7):

```go
func (c *Client) UnaryInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (resp interface{}, err error) {
        defer func() {
            if r := recover(); r != nil {
                err = status.Errorf(codes.Internal, "panic: %v", r)
            }
        }()
        // ... auth check logic ...
        return handler(WithUser(ctx, user), req)
    }
}
```

### 2.3 HTTP middleware — Bearer extraction + WWW-Authenticate

**Authoritative source (AUTH server):**
`/Users/a.popov/Development/TRADING/bots/auth/internal/service/httpauthmw/middleware.go:209-232`

```go
func extractBearer(authz string) (string, bool) {
    authz = strings.TrimLeft(authz, " \t")
    if len(authz) < len(bearerPrefix) || !strings.EqualFold(authz[:len(bearerPrefix)], bearerPrefix) {
        return "", false
    }
    token := authz[len(bearerPrefix):]
    token = strings.TrimSpace(token)
    if token == "" || strings.ContainsAny(token, " \t\r\n") || len(token) > maxTokenLen {
        return "", false
    }
    return token, true
}
```

This is more robust than the ADR's `strings.TrimPrefix(authHeader, "Bearer ")` — it handles
case-insensitive Bearer, leading whitespace, internal whitespace, and length cap. Recommend
adopting this version as the private `bearerToken(authz string) (string, bool)` helper.

`WWW-Authenticate` challenge value used in auth server:
```
Bearer realm="auth", charset="UTF-8"
```
ADR §A10 uses `Bearer realm="api"` — use ADR's value (simpler, no charset needed for RFC 6750).
On 401 for invalid_token: `Bearer realm="api", error="invalid_token"` (ADR).

For 503 (FailClosed): `Retry-After: 5` + `http.Error(w, "auth unavailable", 503)`.
For 429 (RateLimited): `Retry-After: <seconds>` if `RetryAfter(err) > 0`.
The auth server middleware does NOT handle 503/429 — write fresh per ADR §A10.

### 2.4 `metadata.FromIncomingContext` + "authorization" extraction

**Signal-analyzer pattern** (line 26 in interceptors.go):
```go
if md, ok := metadata.FromIncomingContext(ctx); ok {
    if reqIDs := md.Get("x-request-id"); len(reqIDs) > 0 { ... }
}
```

For SDK `UnaryInterceptor` the ADR §A11 flow is:
```go
md, ok := metadata.FromIncomingContext(ctx)
if !ok {
    return nil, status.Error(codes.Unauthenticated, "missing metadata")
}
authValues := md.Get("authorization")  // lowercase — gRPC normalises to lowercase
if len(authValues) == 0 {
    return nil, status.Error(codes.Unauthenticated, "missing authorization")
}
token, ok := bearerToken(authValues[0])
if !ok {
    return nil, status.Error(codes.Unauthenticated, "invalid authorization header")
}
```

**Important:** `md.Get("authorization")` is lowercase. gRPC metadata keys are normalised to
lowercase on the wire AND in `metadata.MD`. Using `"Authorization"` (capital A) would still
work because `metadata.MD.Get` normalises the key — but use lowercase by convention (ADR §A11).

### 2.5 `crypto/sha256` + `hex.EncodeToString` token hashing

**Exact pattern from auth server:**
`/Users/a.popov/Development/TRADING/bots/auth/internal/service/tokenmanager/generate_refresh_token.go:20-21`

```go
hash := sha256.Sum256([]byte(rawToken))
tokenHash = hex.EncodeToString(hash[:])
```

`sha256.Sum256` returns `[32]byte`. Slice with `[:]` to get `[]byte` for `hex.EncodeToString`.
Private helper for SDK:

```go
func hashToken(token string) string {
    h := sha256.Sum256([]byte(token))
    return hex.EncodeToString(h[:])
}
```

No existing SDK usage; write fresh. Pattern confirmed correct from auth server.

### 2.6 `golang.org/x/sync/singleflight` — availability

TASK-001 spec (`task_management/tasks/REQ-001-TASK-001.md:21`) explicitly requires adding
`golang.org/x/sync` to `go.mod`. The current `go.mod` has no deps at all — TASK-001 pins it.
TASK-003 does NOT use singleflight (that is TASK-004). Confirm `golang.org/x/sync` is in `go.mod`
after TASK-001 before TASK-003 starts.

### 2.7 `google.golang.org/grpc/status` + `grpc/codes` in server interceptors

Signal-analyzer at line 114:
```go
err = grpcstatus.Error(codes.Internal, "internal app_error")
```
(alias `grpcstatus "google.golang.org/grpc/status"`)

For SDK interceptor error returns, use `status.Error(codes.Unauthenticated, "...")` etc.
Pattern for all cases:
- Missing metadata / missing Bearer: `status.Error(codes.Unauthenticated, "...")`
- `ErrUnavailable` + FailClosed: `status.Error(codes.Unavailable, "auth unavailable")`
- `ErrUnavailable` + FailOpen: call `handler(WithUser(ctx, User{}), req)` (no error)
- `ErrRateLimited`: `status.Error(codes.ResourceExhausted, "rate limited")`
- `ErrInvalidToken|ErrExpired|ErrBlocked`: `status.Error(codes.Unauthenticated, err.Error())`
- panic recovered: `status.Errorf(codes.Internal, "panic: %v", r)`

### 2.8 Benchmark file conventions

**Found in:**
- `/Users/a.popov/Development/TRADING/bots/invest/pkg/generic/queue_with_pool/queue_test.go:27`
- `/Users/a.popov/Development/GOLANG/fundingbot/pkg/generic/observable/observable_benchmark_test.go:22`

Convention: benchmarks live in `_test.go` files (not separate `_bench_test.go`), in the same
package, using standard `func BenchmarkX(b *testing.B)` + `b.ResetTimer()` + `b.ReportAllocs()`.
File can be named `memory_bench_test.go` or included in `memory_test.go`.

For `BenchmarkValidateCache_Hit`:
```go
func BenchmarkValidateCache_Hit(b *testing.B) {
    ctx := context.Background()
    c := NewMemoryCache(ctx, time.Minute)
    c.Set(ctx, "key", User{UUID: "u1", Role: "viewer"}, time.Minute)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        c.Get(ctx, "key")
    }
    b.ReportAllocs()
}
```

The `< 1µs/op` target is achievable: `sync.Map.Load` is ~50–100ns on modern hardware.

---

## 3. Proto Contract Specifics

### `ValidateTokenRequest` (auth_service.pb.go:464-503)

| Go field | Type | Proto name |
|----------|------|------------|
| `AccessToken` | `string` | `access_token` |

Constructor: `&auth_v1.ValidateTokenRequest{AccessToken: accessToken}`

### `ValidateTokenResponse` (auth_service.pb.go:508-574)

| Go field | Type | Proto name | Notes |
|----------|------|------------|-------|
| `IsValid` | `bool` | `is_valid` | false = auth failed |
| `UserUuid` | `string` | `user_uuid` | populated only when `IsValid=true` |
| `Role` | `string` | `role` | populated only when `IsValid=true` |
| `ErrorReason` | `string` | `error_reason` | populated only when `IsValid=false` |

**NO `Email` field** — confirmed. `User.Email` is always `""` after `ValidateAccessToken`.
**NO `ExpiresAt` field** — confirmed. `ValidateTokenResponse` has 4 fields only.
This means `min(cfg.ValidateCacheTTL, time-to-token-expiry)` collapses to just `cfg.ValidateCacheTTL`.
The ADR note about "min()" is aspirational; in v0.1.0 use only `cfg.ValidateCacheTTL`.

### `ErrorReason` string values (from auth server `dto.go:48`)

Documented by auth server: `"expired"`, `"invalid_signature"`, `"invalid_token_version"`,
`"blocked"`, `"user_not_found"`, `"internal_error"`, `"empty_token"`.

`mapErrorReason` switch:
```go
func mapErrorReason(reason string) error {
    switch reason {
    case "expired", "invalid_token_version":
        return ErrExpired
    case "blocked":
        return ErrBlocked
    default: // "invalid_signature", "user_not_found", "internal_error", "empty_token", ""
        return ErrInvalidToken
    }
}
```

### `AuthServiceClient.ValidateToken` signature (auth_service_grpc.pb.go:58)

```go
ValidateToken(ctx context.Context, in *ValidateTokenRequest, opts ...grpc.CallOption) (*ValidateTokenResponse, error)
```

Full method name constant: `AuthService_ValidateToken_FullMethodName = "/auth.v1.AuthService/ValidateToken"` (line 26).

---

## 4. Gotchas

### G1. Cache interface `any` vs `User` mismatch

TASK-002 PRP declares `Cache` interface with `user any` (line 638–644 of PRP), because `User`
type didn't exist in TASK-002. TASK-003 adds `User` type. The `Cache` interface **must be updated**
in `lib/client.go` to use `User` instead of `any`:

```go
// TASK-002 stub (change in TASK-003):
Get(ctx context.Context, key string) (user any, ok bool)

// TASK-003 target:
Get(ctx context.Context, key string) (user User, ok bool)
```

This is a compile-time change; since `User` is now defined, updating the interface is safe.
`MemoryCache.Get` stores `entry{user User, ...}` — types align with updated interface.

### G2. EC-0034 mismatch: TASK cites it but means sync.Map

`coding_scope.md` lists EC-0034 ("Use RWMutex for read-heavy workloads") as triggered, but ADR
and TASK-003 spec both say use `sync.Map`. These are incompatible advice. Resolution: `sync.Map`
is the correct choice here because:
- Cache is write-once-read-many per key (token hashes are immutable),
- `sync.Map` is optimized for this exact pattern ("entries are only written once but read many times"),
- EC-0034 applies when protecting general mutable maps with `RWMutex`.

The rule citation is a TASK-spec artifact; follow ADR §A2 (`sync.Map`) and note the discrepancy.

### G3. `cleanupInterval = 0` panics `time.NewTicker`

When `ValidateCacheTTL = 0` (cache disabled), `New` passes `cfg.ValidateCacheTTL/2 = 0` to
`NewMemoryCache`. `time.NewTicker(0)` panics. The ADR Blueprint has a guard:
```go
if cleanupInterval > 0 {
    go c.cleanupLoop(bgCtx, cleanupInterval)
}
```
**Must include this guard.**

### G4. `sync.Map.Range` during concurrent `Delete` is safe

Go's `sync.Map` documentation guarantees Range does not block concurrent Store/Delete.
Deleting within Range callback is explicitly safe. No special locking needed in `cleanupLoop`.

### G5. Bearer extraction case-sensitivity

`strings.TrimPrefix(authHeader, "Bearer ")` (ADR pseudocode) is case-sensitive. RFC 7235
says scheme is case-insensitive. The auth server uses `strings.EqualFold`. Recommend the
`EqualFold` version from `httpauthmw/middleware.go:215`. ADR says "Bearer " with capital B —
accept this simplification for v0.1.0 since all standard HTTP clients send `Bearer` (capital).
At minimum, document the case sensitivity assumption.

### G6. `http.Error` sets Content-Type to text/plain and adds newline

`http.Error(w, msg, code)` writes `msg + "\n"` as body. ADR §A10 explicitly says "не сериализуем
в JSON body — оставляем text/plain". This is correct and intentional.

### G7. bgCtx field in Client (from TASK-002 PRP decision)

TASK-002 PRP (lines 808-812) decided to add `bgCtx context.Context` as a **field** on `Client`
(not just local var) so the cleanupLoop goroutine can use `c.bgCtx` when started in TASK-003.
The `New` function stores `c.bgCtx = bgCtx`. When TASK-003 creates `MemoryCache`, it passes
`c.bgCtx` (not a new context). Verify this in the TASK-002 output code before implementing.

### G8. Cache interface update requires MemoryCache to implement the updated signature

After changing `Cache.Get` from `(user any, ok bool)` to `(user User, ok bool)`, the
`MemoryCache.Get` must match. Also, `Cache.Set` changes from `(user any, ttl)` to `(user User, ttl)`.
The `WithCache` option and `Config.Cache` field automatically benefit since both use the interface.

---

## 5. Recommended Patterns

| Topic | Source | Pattern to use |
|-------|--------|---------------|
| sha256 token hash | `/bots/auth/.../generate_refresh_token.go:20-21` | `sha256.Sum256([]byte(token))` + `hex.EncodeToString(h[:])` |
| Bearer extraction | `/bots/auth/.../httpauthmw/middleware.go:209-232` | Case-insensitive `EqualFold`, TrimSpace, ContainsAny whitespace check |
| Recovery interceptor (named return) | `/bots/signal-analyzer/.../interceptors.go:99-119` | Named `(resp interface{}, err error)` + `defer recover()` sets `err` |
| metadata extraction | `/bots/signal-analyzer/.../interceptors.go:26-29` | `metadata.FromIncomingContext(ctx)` → `md.Get("authorization")` (lowercase) |
| gRPC status errors | `/bots/signal-analyzer/.../interceptors.go:114` | `grpcstatus.Error(codes.Internal, "...")` |
| sync.Map lazy-expire cache | ADR Blueprint (no sibling example) | Write fresh per ADR §A2 pseudocode |
| Benchmark convention | `/bots/invest/.../queue_test.go:27` + `/fundingbot/.../observable_benchmark_test.go:22` | `func BenchmarkX(b *testing.B)` + `b.ResetTimer()` + `b.ReportAllocs()` in `*_test.go` |
| cleanupLoop EC-0012 | ADR Blueprint lines 1281-1298 | Ticker + `select {case <-ctx.Done(): return; case <-t.C: ...}` |

---

## 6. Summary

- **5 files to create:** `lib/internal/cache/memory.go`, `lib/validate.go`, `lib/middleware_http.go`, `lib/interceptor_grpc.go` — plus plumbing updates to `lib/client.go`.
- **Proto confirmed:** `ValidateTokenResponse` has 4 fields (`IsValid`, `UserUuid`, `Role`, `ErrorReason`), no `ExpiresAt`, no `Email`. Cache TTL is always `cfg.ValidateCacheTTL` only.
- **ErrorReason values confirmed** from auth server source: `"expired"`, `"invalid_token_version"` → ErrExpired; `"blocked"` → ErrBlocked; everything else → ErrInvalidToken.
- **Critical action for TASK-003:** update `Cache` interface in `client.go` from `user any` to `user User` (requires `User` type defined in TASK-002's `types.go`/`user.go`).
- **EC-0034 vs sync.Map:** TASK spec cites EC-0034 but ADR requires `sync.Map`. Use `sync.Map` — it is the correct primitive for this write-once-read-many pattern.
- **Benchmark target `< 1µs/op`** on cache hit is achievable; `sync.Map.Load` is ~50–100 ns.
