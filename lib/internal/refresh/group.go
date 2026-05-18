// Package refresh provides a thin wrapper over singleflight.Group with
// mandatory key-forget after each Do call.
//
// Why a wrapper? Plain singleflight.Group keeps the last result until the
// caller invokes Forget. For refresh-token rotation that is wrong: after fn
// returns, the result is single-use (the refresh token has been rotated —
// calling Do again with the SAME key must hit AUTH for a fresh rotation).
// With Forget right after Do, that invariant holds.
package refresh

import "golang.org/x/sync/singleflight"

// Group deduplicates concurrent calls with the same key. After fn completes
// (whether successfully or with error), the key is forgotten so subsequent
// callers run a fresh fn.
//
// Zero value is ready to use.
type Group struct {
	sf singleflight.Group
}

// Do invokes fn at most once per concurrent key. Concurrent callers with the
// same key block on the leader's call; all waiters receive the leader's
// (value, error) pair when fn returns.
//
// Note: singleflight does NOT respect ctx cancellation — if a follower's
// context cancels, the follower still waits until the leader's fn returns.
// For Refresh this is intentional (the rotation must complete to avoid AUTH
// family detection on the next attempt).
//
// The underlying singleflight.Group.Do returns a `shared` bool indicating
// whether more than one caller received the same result. This wrapper does
// not surface it — the public contract is `(any, error)`. If deduplication
// metrics are needed in the future, expose them via a Stats() method.
func (g *Group) Do(key string, fn func() (any, error)) (any, error) {
	v, err, _ := g.sf.Do(key, fn)
	g.sf.Forget(key)
	return v, err
}
