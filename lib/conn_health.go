package authclient

import (
	"google.golang.org/grpc/connectivity"
)

// ConnHealth is a read-only snapshot of the client's underlying gRPC
// connection. It is a by-value copy: it does NOT expose *grpc.ClientConn,
// so external code cannot mutate or close the connection.
//
// The struct is intentionally designed to be extended additively: future
// fields (e.g. LastError, LatencyMs) can be appended without breaking the
// existing snapshot contract.
type ConnHealth struct {
	// State is the current gRPC connectivity state of the connection.
	State connectivity.State

	// Target is the dial-target (host:port) the client connects to.
	Target string
}

// ConnHealth returns a snapshot of the client's gRPC connection state and
// target. It reads ClientConn.GetState() and ClientConn.Target(), both of
// which are O(1) and goroutine-safe per the gRPC concurrency guarantees, so
// ConnHealth performs no network call and is safe for concurrent use.
//
// When the underlying connection is nil (e.g. a Client built without a live
// dial), it reports {State: connectivity.Shutdown, Target: ""}.
//
// The returned value reflects connection-level state only; it is NOT the
// result of an active health-probe RPC against the AUTH server.
func (c *Client) ConnHealth() ConnHealth {
	if c.grpcConn == nil {
		return ConnHealth{State: connectivity.Shutdown, Target: ""}
	}

	return ConnHealth{
		State:  c.grpcConn.GetState(),
		Target: c.grpcConn.Target(),
	}
}
