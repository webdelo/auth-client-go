//go:build integration

package lib_test

import (
	"net"
	"os"
	"testing"
	"time"

	// Blank imports keep direct dependencies pinned until TASK-002+ wires
	// them into real SDK code. Removed once TASK-002 imports auth_v1 and
	// google.golang.org/grpc in production paths, and TASK-004 wires
	// singleflight into the refresh flow.
	_ "github.com/webdelo/auth-proto/go/auth_v1"
	_ "golang.org/x/sync/singleflight"
	_ "google.golang.org/grpc"
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
