// Package authclient is the official Go SDK for the webdelo/auth service.
//
// It is a thin wrapper over the gRPC contract from webdelo/auth-proto with
// non-trivial concerns baked in: a TTL cache for ValidateToken, an HTTP
// middleware, a gRPC unary interceptor, refresh-race serialization via
// singleflight, typed errors, and fail-closed-by-default semantics. The SDK
// has no DI, logger, or metrics dependencies — consumers plug in their own
// via small adapter interfaces (Logger, Cache).
//
// # Quickstart
//
// Create a Client, attach the HTTP middleware to a protected handler, and
// read the authenticated user out of the request context:
//
//	client, err := authclient.New(authclient.Config{Endpoint: "auth:50051"})
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//	mux.Handle("/api/secure", client.HTTPMiddleware(handler))
//
// Inside the handler — the middleware guarantees the user is present, so the
// second return is safe to drop:
//
//	user, _ := authclient.UserFromContext(r.Context())
//	// user.UUID, user.Role are populated
//
// # Related
//
//   - Server: https://github.com/webdelo/auth
//   - gRPC contract: https://github.com/webdelo/auth-proto
//   - SDK versioning policy: see CHANGELOG.md and README.md.
package authclient
