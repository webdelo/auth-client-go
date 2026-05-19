# Example: basic

## What

Minimal happy-path: `New → Login → ValidateAccessToken → Logout → Close`.
Demonstrates the four core operations every SDK consumer eventually calls.
ENV-driven; no flags.

## Prerequisites

- A running AUTH gRPC server (see [webdelo/auth](https://github.com/webdelo/auth)
  or run `make env-up` in this repo to spin one up via docker-compose).
- A test user account in that AUTH instance.

## Run

```sh
export AUTH_ENDPOINT=localhost:50051
export AUTH_TEST_EMAIL=test@example.com
export AUTH_TEST_PASSWORD=secret123
go run ./lib/examples/basic
```

Without environment variables, the binary prints a usage line and exits 0.

## Expected output

```
step 1: New ok (endpoint=localhost:50051)
step 2: Login ok user=<uuid> role=user access=eyJhbGci…
step 3: Validate ok user.uuid=<uuid> user.role=user
step 4: Logout ok
step 5: Close ok (deferred)
```

Tokens are truncated to 8 chars + ellipsis for safety.

## Notes

- The example passes `ClientIP="127.0.0.1"` and a literal User-Agent. In a
  real HTTP-facing service, use `authclient.IPFromRequest(r)` and
  `authclient.UserAgentFromRequest(r)` from a `*http.Request`.
- `User.Email` is empty after `ValidateAccessToken` (proto v0.1.0 limitation).
