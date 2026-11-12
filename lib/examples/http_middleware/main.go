// Package main is the http_middleware authclient SDK example: it wires
// client.HTTPMiddleware onto a protected /api/profile endpoint, exposes
// /auth/login (sets HttpOnly refresh cookie) and /auth/logout (clears it).
// Demonstrates passing a *slog.Logger directly to the SDK and the
// IPFromRequest / UserAgentFromRequest fingerprint helpers.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	authclient "github.com/webdelo/auth-client-go/lib"
)

func main() {
	os.Exit(run())
}

func run() int {
	endpoint := os.Getenv("AUTH_ENDPOINT")
	if endpoint == "" {
		usage()
		return 0
	}

	addr := getenvOr("HTTP_ADDR", ":8080")
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client, err := authclient.New(authclient.Config{
		Endpoint: endpoint,
		Logger:   logger,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "authclient.New: %v\n", err)
		return 1
	}

	defer func() {
		if cerr := client.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "client.Close: %v\n", cerr)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", loginHandler(client))
	mux.HandleFunc("POST /auth/logout", logoutHandler(client))
	mux.Handle("GET /api/profile", client.HTTPMiddleware(http.HandlerFunc(profileHandler)))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "http server listening", "addr", addr)
		if lerr := srv.ListenAndServe(); lerr != nil && !errors.Is(lerr, http.ErrServerClosed) {
			serverErr <- lerr
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		logger.InfoContext(ctx, "shutdown signal received")
	case lerr := <-serverErr:
		if lerr != nil {
			fmt.Fprintf(os.Stderr, "http listen: %v\n", lerr)
			return 1
		}
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if serr := srv.Shutdown(shutCtx); serr != nil {
		fmt.Fprintf(os.Stderr, "http shutdown: %v\n", serr)
	}
	return 0
}

func loginHandler(c *authclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		out, err := c.Login(r.Context(), authclient.LoginInput{
			Email:     req.Email,
			Password:  req.Password,
			ClientIP:  authclient.IPFromRequest(r),
			UserAgent: authclient.UserAgentFromRequest(r),
		})
		switch {
		case errors.Is(err, authclient.ErrInvalidToken):
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		case errors.Is(err, authclient.ErrRateLimited):
			if d := authclient.RetryAfter(err); d > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(d.Seconds()))))
			}
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		case errors.Is(err, authclient.ErrUnavailable):
			http.Error(w, "auth unavailable", http.StatusServiceUnavailable)
			return
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		authclient.SetRefreshCookie(w, out.RefreshToken)
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":       out.AccessToken,
			"expires_in_seconds": int(out.AccessExpiresIn.Seconds()),
			"user": map[string]string{
				"uuid":  out.User.UUID,
				"role":  string(out.User.Role),
				"email": out.User.Email,
			},
		})
	}
}

func logoutHandler(c *authclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("refresh_token")
		if err != nil {
			http.Error(w, "no refresh cookie", http.StatusBadRequest)
			return
		}
		if lerr := c.Logout(r.Context(), cookie.Value); lerr != nil && !errors.Is(lerr, authclient.ErrInvalidToken) {
			http.Error(w, lerr.Error(), http.StatusInternalServerError)
			return
		}
		authclient.ClearRefreshCookie(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := authclient.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "no user", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"uuid": user.UUID,
		"role": string(user.Role),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// best-effort logging; headers already written
		fmt.Fprintf(os.Stderr, "writeJSON encode: %v\n", err)
	}
}

func getenvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return def
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: AUTH_ENDPOINT=host:port [HTTP_ADDR=:8080] http_middleware")
	fmt.Fprintln(os.Stderr, "see lib/examples/http_middleware/README.md")
}
