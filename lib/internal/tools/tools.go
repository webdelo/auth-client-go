//go:build tools

// Package tools keeps build-time-only dependencies pinned in go.mod.
// The build tag "tools" prevents the package from being compiled in normal
// builds; go mod tidy still records the imports.
package tools

import (
	// Pinned for the upcoming refresh singleflight wiring; remove once a real
	// import path inside authclient depends on it.
	_ "golang.org/x/sync/singleflight"
)
