//go:build integration

package authclient_test

import (
	"net"
	"os"
	"testing"
	"time"

	// Blank import keeps the singleflight dependency pinned until TASK-004
	// wires it into the refresh flow. auth_v1 and grpc are direct imports of
	// the SDK now (TASK-002).
	_ "golang.org/x/sync/singleflight"
)

func TestMain(m *testing.M) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestAuthEndpointReachable(t *testing.T) {
	endpoint := os.Getenv("AUTH_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:50051"
	}
	conn, err := net.DialTimeout("tcp", endpoint, 5*time.Second)
	if err != nil {
		t.Fatalf("dial AUTH at %s: %v", endpoint, err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close conn: %v", err)
	}
}
