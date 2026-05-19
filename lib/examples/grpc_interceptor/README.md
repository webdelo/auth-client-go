# Example: grpc_interceptor

## What

Single-process gRPC server + client demo of `client.UnaryInterceptor()`:

- Server registers `health.NewServer()` (from `google.golang.org/grpc/health`)
  and wires the SDK's unary interceptor.
- An in-process client fires three RPCs and the program exits 0 on success.

`/grpc.health.v1.Health/Check` and `/Watch` are in the SDK's default open-methods
registry (no auth needed). `/grpc.health.v1.Health/List` is **not** in that
registry, so the interceptor enforces a valid Bearer token in gRPC metadata.

## Prerequisites

- A reachable AUTH endpoint and a test user (the example calls `client.Login`
  to obtain a real access token, then uses it in metadata).

## Run

```sh
export AUTH_ENDPOINT=localhost:50051
export AUTH_TEST_EMAIL=test@example.com
export AUTH_TEST_PASSWORD=secret123
go run ./lib/examples/grpc_interceptor
```

Without env vars, the binary prints a usage line and exits 0.

## Expected output

```
RPC1 Health/Check (no auth, default openMethod) → SERVING
RPC2 Health/List (with Bearer) → 1 statuses
RPC3 Health/List (no metadata) → Unauthenticated (as expected)
all 3 RPCs passed
```

## Notes

- `Health/List` is the demo "protected" method because `Check` and `Watch` are
  unconditionally open in the SDK's default registry. Add your own open methods
  with `authclient.WithOpenMethods("/your.Svc/Method")`.
- The example listens on an ephemeral port (`127.0.0.1:0`) — multiple copies
  can run in parallel without conflict.
