// Package main is the basic authclient SDK example: it logs in with credentials
// from environment variables, validates the issued access token, and logs out.
// Run without any environment variables to see a usage line.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
	fmt.Printf("step 1: New ok (endpoint=%s)\n", endpoint)

	out, err := client.Login(ctx, authclient.LoginInput{
		Email:     email,
		Password:  password,
		ClientIP:  "127.0.0.1", // demo only; use authclient.IPFromRequest(r) in real HTTP handler
		UserAgent: "authclient-basic-example/0.1",
	})
	if err != nil {
		switch {
		case errors.Is(err, authclient.ErrInvalidToken):
			fmt.Fprintln(os.Stderr, "Login: wrong email or password")
		case errors.Is(err, authclient.ErrRateLimited):
			fmt.Fprintf(os.Stderr, "Login: rate limited, retry after %s\n", authclient.RetryAfter(err))
		default:
			fmt.Fprintf(os.Stderr, "Login: %v\n", err)
		}
		return 1
	}
	fmt.Printf("step 2: Login ok user=%s role=%s access=%s\n",
		out.User.UUID, out.User.Role, safeTrunc(out.AccessToken, 8))

	user, err := client.ValidateAccessToken(ctx, out.AccessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ValidateAccessToken: %v\n", err)
		return 1
	}
	fmt.Printf("step 3: Validate ok user.uuid=%s user.role=%s\n", user.UUID, user.Role)

	if err := client.Logout(ctx, out.RefreshToken); err != nil {
		fmt.Fprintf(os.Stderr, "Logout: %v\n", err)
		return 1
	}
	fmt.Println("step 4: Logout ok")
	fmt.Println("step 5: Close ok (deferred)")
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
	fmt.Fprintln(os.Stderr, "usage: AUTH_ENDPOINT=host:port AUTH_TEST_EMAIL=... AUTH_TEST_PASSWORD=... basic")
	fmt.Fprintln(os.Stderr, "see lib/examples/basic/README.md")
}

func safeTrunc(s string, n int) string {
	if n > len(s) {
		n = len(s)
	}

	return s[:n] + "…"
}
