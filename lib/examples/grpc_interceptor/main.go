// Package main is the grpc_interceptor authclient SDK example: it starts a
// gRPC server protected by client.UnaryInterceptor() and exercises three
// RPCs from an in-process client:
//
//  1. /grpc.health.v1.Health/Check without metadata    - skipped (default openMethods) -> SERVING
//  2. /grpc.health.v1.Health/List with a valid Bearer  - interceptor accepts -> success
//  3. /grpc.health.v1.Health/List without metadata     - interceptor returns codes.Unauthenticated
//
// Run without AUTH_ENDPOINT/AUTH_TEST_EMAIL/AUTH_TEST_PASSWORD to see a usage
// line and exit 0.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authclient "github.com/webdelo/auth-client-go/lib"
)

func main() {
	os.Exit(run())
}

func run() int {
	endpoint, email, password, ok := readEnv()
	if !ok {
		usage()
		return 0
	}

	client, err := authclient.New(authclient.Config{Endpoint: endpoint})
	if err != nil {
		fmt.Fprintf(os.Stderr, "authclient.New: %v\n", err)
		return 1
	}

	defer func() {
		if cerr := client.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "client.Close: %v\n", cerr)
		}
	}()

	loginCtx, cancelLogin := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelLogin()

	out, err := client.Login(loginCtx, authclient.LoginInput{
		Email:     email,
		Password:  password,
		ClientIP:  "127.0.0.1",
		UserAgent: "authclient-grpc-example/0.1",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login: %v\n", err)
		return 1
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "net.Listen: %v\n", err)
		return 1
	}

	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(client.UnaryInterceptor()))
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcSrv, healthSrv)

	go func() {
		if serr := grpcSrv.Serve(lis); serr != nil {
			fmt.Fprintf(os.Stderr, "grpc.Serve: %v\n", serr)
		}
	}()
	defer grpcSrv.GracefulStop()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "grpc.NewClient: %v\n", err)
		return 1
	}

	defer func() {
		if cerr := conn.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "grpc conn close: %v\n", cerr)
		}
	}()

	hc := grpc_health_v1.NewHealthClient(conn)

	rpcCtx, cancelRPC := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRPC()

	resp1, err := hc.Check(rpcCtx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "RPC1 Health/Check failed (expected success): %v\n", err)
		return 1
	}
	fmt.Printf("RPC1 Health/Check (no auth, default openMethod) → %s\n", resp1.GetStatus())

	md := metadata.Pairs("authorization", "Bearer "+out.AccessToken)
	authCtx := metadata.NewOutgoingContext(rpcCtx, md)
	resp2, err := hc.List(authCtx, &grpc_health_v1.HealthListRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "RPC2 Health/List with token failed (expected success): %v\n", err)
		return 1
	}
	fmt.Printf("RPC2 Health/List (with Bearer) → %d statuses\n", len(resp2.GetStatuses()))

	_, err = hc.List(rpcCtx, &grpc_health_v1.HealthListRequest{})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		fmt.Fprintf(os.Stderr, "RPC3 Health/List without token: expected Unauthenticated, got %v\n", err)
		return 1
	}
	fmt.Printf("RPC3 Health/List (no metadata) → %s (as expected)\n", st.Code())

	if lerr := client.Logout(rpcCtx, out.RefreshToken); lerr != nil {
		fmt.Fprintf(os.Stderr, "warn: Logout failed: %v\n", lerr)
	}

	fmt.Println("all 3 RPCs passed")
	return 0
}

func readEnv() (endpoint, email, password string, ok bool) {
	endpoint = os.Getenv("AUTH_ENDPOINT")
	email = os.Getenv("AUTH_TEST_EMAIL")
	password = os.Getenv("AUTH_TEST_PASSWORD")
	ok = endpoint != "" && email != "" && password != ""

	return endpoint, email, password, ok
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: AUTH_ENDPOINT=host:port AUTH_TEST_EMAIL=... AUTH_TEST_PASSWORD=... grpc_interceptor")
	fmt.Fprintln(os.Stderr, "see lib/examples/grpc_interceptor/README.md")
}
