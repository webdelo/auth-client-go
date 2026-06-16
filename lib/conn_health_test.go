package authclient

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// waitForState blocks until conn reaches want or the test deadline elapses.
// It loops over WaitForStateChange so intermediate transitions (e.g.
// Connecting → TransientFailure) do not cause a false timeout.
func waitForState(t *testing.T, conn *grpc.ClientConn, want connectivity.State) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		s := conn.GetState()
		if s == want {
			return
		}
		if !conn.WaitForStateChange(ctx, s) {
			t.Fatalf("timeout waiting for state %v, last observed %v", want, conn.GetState())
		}
	}
}

// newGatedConn returns a ClientConn whose dialer blocks until release is
// closed, plus a live bufconn server it dials. While release is open the conn
// stays in CONNECTING after Connect(); closing release lets it reach READY.
func newGatedConn(t *testing.T) (conn *grpc.ClientConn, release chan struct{}) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	release = make(chan struct{})
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		<-release
		return lis.DialContext(ctx)
	}

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return conn, release
}

func TestClient_ConnHealth_NilConn(t *testing.T) {
	t.Parallel()

	c := &Client{} // grpcConn is nil

	got := c.ConnHealth()

	assert.Equal(t, connectivity.Shutdown, got.State, "nil conn must report Shutdown")
	assert.Empty(t, got.Target, "nil conn must report empty target")
}

func TestClient_ConnHealth_Idle(t *testing.T) {
	t.Parallel()

	conn, err := grpc.NewClient(
		"passthrough:///localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	c := &Client{grpcConn: conn}

	got := c.ConnHealth()

	assert.Equal(t, connectivity.Idle, got.State, "fresh conn (no Connect) must be Idle")
	assert.Equal(t, "passthrough:///localhost:50051", got.Target)
	assert.Equal(t, conn.Target(), got.Target, "Target must mirror ClientConn.Target()")
}

func TestClient_ConnHealth_Shutdown(t *testing.T) {
	t.Parallel()

	conn, err := grpc.NewClient(
		"passthrough:///localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	c := &Client{grpcConn: conn}

	got := c.ConnHealth()

	assert.Equal(t, connectivity.Shutdown, got.State, "closed conn must report Shutdown")
}

func TestClient_ConnHealth_ConnectingThenReady(t *testing.T) {
	t.Parallel()

	conn, release := newGatedConn(t)
	c := &Client{grpcConn: conn}

	// Trigger exit-idle. The gated dialer blocks, so the conn parks in
	// CONNECTING until we release it.
	conn.Connect()
	waitForState(t, conn, connectivity.Connecting)

	got := c.ConnHealth()
	assert.Equal(t, connectivity.Connecting, got.State, "blocked dialer must hold conn in Connecting")
	assert.Equal(t, "passthrough:///bufnet", got.Target)

	// Release the dialer and let the handshake complete.
	close(release)
	waitForState(t, conn, connectivity.Ready)

	got = c.ConnHealth()
	assert.Equal(t, connectivity.Ready, got.State, "released dialer must let conn reach Ready")
	assert.Equal(t, conn.Target(), got.Target)
}

func TestClient_ConnHealth_TransientFailure(t *testing.T) {
	t.Parallel()

	dialer := func(_ context.Context, _ string) (net.Conn, error) {
		return nil, errors.New("connection refused")
	}

	conn, err := grpc.NewClient(
		"passthrough:///unreachable",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	c := &Client{grpcConn: conn}

	conn.Connect()
	waitForState(t, conn, connectivity.TransientFailure)

	got := c.ConnHealth()
	assert.Equal(t, connectivity.TransientFailure, got.State, "failing dialer must yield TransientFailure")
	assert.Equal(t, "passthrough:///unreachable", got.Target)
}

func TestClient_ConnHealth_Concurrent(t *testing.T) {
	t.Parallel()

	conn, err := grpc.NewClient(
		"passthrough:///localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	c := &Client{grpcConn: conn}

	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			got := c.ConnHealth()
			assert.Equal(t, "passthrough:///localhost:50051", got.Target)
		}()
	}
	wg.Wait()
}
