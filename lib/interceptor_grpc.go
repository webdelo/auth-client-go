package authclient

import (
	"context"
	"errors"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryInterceptor returns a gRPC UnaryServerInterceptor that:
//   - skips authentication for methods registered via WithOpenMethods
//     (default: /grpc.health.v1.Health/Check, /grpc.health.v1.Health/Watch);
//   - extracts the Bearer token from the incoming metadata "authorization";
//   - validates via ValidateAccessToken;
//   - injects User into the handler context via WithUser on success;
//   - maps SDK errors to gRPC codes (Unauthenticated / Unavailable /
//     ResourceExhausted), honoring FailMode for ErrUnavailable;
//   - recovers from handler panics, logs them, and returns codes.Internal.
//
// The named return values (resp, err) are required: defer recover assigns
// to err, which only propagates to the caller when err is a named return.
func (c *Client) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				c.logger.Error(ctx, "authclient: grpc handler panic", nil,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, "internal error")
				resp = nil
			}
		}()

		if _, open := c.openMethods[info.FullMethod]; open {
			return handler(ctx, req)
		}

		md, mdOK := metadata.FromIncomingContext(ctx)
		if !mdOK {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}

		authValues := md.Get("authorization")
		if len(authValues) == 0 {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}

		token, tokOK := bearerToken(authValues[0])
		if !tokOK {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}

		user, vErr := c.ValidateAccessToken(ctx, token)
		switch {
		case errors.Is(vErr, ErrUnavailable):
			if c.failMode == FailOpen {
				c.logger.Warn(ctx, "authclient: AUTH unavailable, fail-open pass-through")
				return handler(WithUser(ctx, User{}), req)
			}
			return nil, status.Error(codes.Unavailable, "auth unavailable")

		case errors.Is(vErr, ErrRateLimited):
			return nil, status.Error(codes.ResourceExhausted, "rate limited")

		case vErr != nil: // ErrInvalidToken / ErrExpired / ErrBlocked
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}

		return handler(WithUser(ctx, user), req)
	}
}
