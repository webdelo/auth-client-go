//go:build integration

package authclient_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	authclient "github.com/webdelo/auth-client-go/lib"
)

const (
	testIP        = "203.0.113.55"
	testUserAgent = "auth-client-go-integration-test/0.1.0"
)

// newIntegrationClient builds a Client wired to the local AUTH from env.
func newIntegrationClient(t *testing.T) *authclient.Client {
	t.Helper()
	endpoint := os.Getenv("AUTH_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:50051"
	}
	c, err := authclient.New(authclient.Config{Endpoint: endpoint})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// seedTestUser inserts a fresh user into AUTH's Postgres directly. Uses
// bcrypt cost=4 (test-fast). Schedules DELETE via t.Cleanup.
func seedTestUser(t *testing.T, email, password string) {
	t.Helper()
	dsn := os.Getenv("AUTH_DB_DSN")
	require.NotEmpty(t, dsn, "AUTH_DB_DSN must be set in dev/deployments/test.env")

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), `
		INSERT INTO users (uuid, email, password_hash, role, is_blocked, token_version, created_at)
		VALUES (gen_random_uuid(), $1, $2, 'user', false, 0, NOW())
	`, email, string(hash))
	require.NoError(t, err, "INSERT user")

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM users WHERE email = $1`, email)
	})
}

// uniqueEmail returns a per-test email with a unique suffix.
func uniqueEmail(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test_%s_%d@example.com",
		strings.ReplaceAll(t.Name(), "/", "_"),
		time.Now().UnixNano())
}

func TestLogin_Happy(t *testing.T) {
	t.Parallel()

	email := uniqueEmail(t)
	password := "hunter2-integration"
	c := newIntegrationClient(t)
	seedTestUser(t, email, password)

	out, err := c.Login(context.Background(), authclient.LoginInput{
		Email:     email,
		Password:  password,
		ClientIP:  testIP,
		UserAgent: testUserAgent,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, out.AccessToken, "AccessToken must be non-empty")
	assert.NotEmpty(t, out.RefreshToken, "RefreshToken must be non-empty")
	assert.Greater(t, out.AccessExpiresIn, time.Duration(0), "AccessExpiresIn must be positive")
	assert.Greater(t, out.RefreshExpiresIn, time.Duration(0), "RefreshExpiresIn must be positive")
	assert.Equal(t, email, out.User.Email)
	assert.NotEmpty(t, out.User.UUID, "User.UUID must be non-empty")
}

func TestRefresh_HappyAndRotation(t *testing.T) {
	t.Parallel()

	email := uniqueEmail(t)
	password := "hunter2-integration"
	c := newIntegrationClient(t)
	seedTestUser(t, email, password)

	initial, err := c.Login(context.Background(), authclient.LoginInput{
		Email:     email,
		Password:  password,
		ClientIP:  testIP,
		UserAgent: testUserAgent,
	})
	require.NoError(t, err, "login before refresh")

	refreshed, err := c.Refresh(context.Background(), initial.RefreshToken, testIP, testUserAgent)
	require.NoError(t, err, "first refresh")

	assert.NotEqual(t, initial.RefreshToken, refreshed.RefreshToken,
		"rotated refresh token must differ from original")
	assert.NotEqual(t, initial.AccessToken, refreshed.AccessToken,
		"new access token must differ from original")

	// Reuse the old (now-invalidated) refresh token — must fail.
	_, err = c.Refresh(context.Background(), initial.RefreshToken, testIP, testUserAgent)
	require.Error(t, err, "reusing old refresh token must return an error")
	assert.True(t, errors.Is(err, authclient.ErrInvalidToken),
		"reused refresh token must return ErrInvalidToken, got: %v", err)
}

func TestRefresh_Race(t *testing.T) {
	// Not parallel — stress test; isolation from other integration tests matters.

	email := uniqueEmail(t)
	password := "hunter2-integration"
	c := newIntegrationClient(t)
	seedTestUser(t, email, password)

	loginOut, err := c.Login(context.Background(), authclient.LoginInput{
		Email:     email,
		Password:  password,
		ClientIP:  testIP,
		UserAgent: testUserAgent,
	})
	require.NoError(t, err, "login before race refresh")

	const goroutines = 10

	type result struct {
		out authclient.RefreshOutput
		err error
	}

	results := make([]result, goroutines)
	start := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i // capture loop var (EC-0091)
		go func() {
			defer wg.Done()
			<-start
			out, err := c.Refresh(context.Background(), loginOut.RefreshToken, testIP, testUserAgent)
			results[i] = result{out: out, err: err}
		}()
	}

	close(start)
	wg.Wait()

	// All goroutines must succeed (singleflight collapses concurrent calls).
	for i, r := range results {
		require.NoError(t, r.err, "goroutine %d must not error", i)
	}

	// All must have received the same rotated refresh token (singleflight dedup).
	firstToken := results[0].out.RefreshToken
	assert.NotEmpty(t, firstToken, "rotated refresh token must be non-empty")
	for i, r := range results[1:] {
		assert.Equal(t, firstToken, r.out.RefreshToken,
			"goroutine %d token must equal goroutine 0 token (singleflight)", i+1)
	}

	// The rotated token from the fan-out must still be usable.
	_, err = c.Refresh(context.Background(), firstToken, testIP, testUserAgent)
	require.NoError(t, err, "post-race refresh with rotated token must succeed")
}

func TestLogout(t *testing.T) {
	t.Parallel()

	email := uniqueEmail(t)
	password := "hunter2-integration"
	c := newIntegrationClient(t)
	seedTestUser(t, email, password)

	loginOut, err := c.Login(context.Background(), authclient.LoginInput{
		Email:     email,
		Password:  password,
		ClientIP:  testIP,
		UserAgent: testUserAgent,
	})
	require.NoError(t, err, "login before logout")

	err = c.Logout(context.Background(), loginOut.RefreshToken)
	require.NoError(t, err, "logout must succeed")

	// After logout, the refresh token must be invalid.
	_, err = c.Refresh(context.Background(), loginOut.RefreshToken, testIP, testUserAgent)
	require.Error(t, err, "refresh after logout must return an error")
	assert.True(t, errors.Is(err, authclient.ErrInvalidToken),
		"refresh after logout must return ErrInvalidToken, got: %v", err)
}

func TestChangePassword(t *testing.T) {
	t.Parallel()

	email := uniqueEmail(t)
	oldPassword := "old-hunter2-integration"
	newPassword := "new-hunter2-integration"
	c := newIntegrationClient(t)
	seedTestUser(t, email, oldPassword)

	loginOut, err := c.Login(context.Background(), authclient.LoginInput{
		Email:     email,
		Password:  oldPassword,
		ClientIP:  testIP,
		UserAgent: testUserAgent,
	})
	require.NoError(t, err, "login with old password")

	err = c.ChangePassword(context.Background(), loginOut.AccessToken, oldPassword, newPassword)
	require.NoError(t, err, "change_password must succeed")

	// Old password must no longer work.
	_, err = c.Login(context.Background(), authclient.LoginInput{
		Email:     email,
		Password:  oldPassword,
		ClientIP:  testIP,
		UserAgent: testUserAgent,
	})
	require.Error(t, err, "login with old password after change must fail")

	// New password must work.
	_, err = c.Login(context.Background(), authclient.LoginInput{
		Email:     email,
		Password:  newPassword,
		ClientIP:  testIP,
		UserAgent: testUserAgent,
	})
	require.NoError(t, err, "login with new password must succeed")
}

func TestGetCurrentUser_HasEmail(t *testing.T) {
	t.Parallel()

	email := uniqueEmail(t)
	password := "hunter2-integration"
	c := newIntegrationClient(t)
	seedTestUser(t, email, password)

	loginOut, err := c.Login(context.Background(), authclient.LoginInput{
		Email:     email,
		Password:  password,
		ClientIP:  testIP,
		UserAgent: testUserAgent,
	})
	require.NoError(t, err, "login before get_current_user")

	user, err := c.GetCurrentUser(context.Background(), loginOut.AccessToken)
	require.NoError(t, err, "get_current_user must succeed")
	assert.Equal(t, email, user.Email)
	assert.NotEmpty(t, user.UUID, "User.UUID must be non-empty")
}
