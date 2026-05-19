# Example: http_middleware

## What

Single-file `net/http` server with:

- `POST /auth/login` — JSON in, sets HttpOnly refresh cookie, returns access token.
- `POST /auth/logout` — clears the refresh cookie via `client.Logout`.
- `GET /api/profile` — protected by `client.HTTPMiddleware`; reads `UserFromContext`.

Shows the `slog` → `authclient.Logger` adapter pattern and the
`IPFromRequest` / `UserAgentFromRequest` fingerprint helpers.

## Prerequisites

A reachable AUTH endpoint (`AUTH_ENDPOINT=host:port`).

## Run

```sh
export AUTH_ENDPOINT=localhost:50051
go run ./lib/examples/http_middleware   # listens on :8080; HTTP_ADDR overrides

CREDS='{"email":"test@example.com","password":"secret123"}'
ACCESS=$(curl -sS -X POST http://localhost:8080/auth/login \
    -H 'Content-Type: application/json' -c cookies.txt -d "$CREDS" | jq -r .access_token)
curl -sS http://localhost:8080/api/profile -H "Authorization: Bearer $ACCESS"
curl -sS -X POST http://localhost:8080/auth/logout -b cookies.txt
```

Without `AUTH_ENDPOINT`, the binary prints a usage line and exits 0.

## Expected output

`/auth/login` returns `{"access_token":"...", "expires_in_seconds": 900, "user":{"uuid":"...","role":"user","email":"..."}}`.
`/api/profile` returns `{"uuid":"...","role":"user"}`.
`/auth/logout` returns `204 No Content` and sends `Set-Cookie` with `MaxAge=-1`.

## Notes

- `IPFromRequest` may be spoofed without a trusted proxy. In production, put
  this service behind nginx/Cloudflare and configure XFF correctly.
- `SetRefreshCookie` sets `Path=/auth/refresh`. The browser scopes the cookie
  to that path; the `/auth/logout` route reads the cookie because curl sends
  it explicitly. In a real client, scope your logout to the same path or use
  `WithCookiePath("/")` symmetrically.
