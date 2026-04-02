// Package snap defines the core abstractions for state-aware checkpoints.
package snap

import (
	"context"
	"io"
)

// DriverPriority determines execution order during save and restore.
// Lower numeric values execute first. The Orchestrator sorts all
// active drivers by priority before iterating.
type DriverPriority int

const (
	// PriorityEnvironment is for drivers that manage configuration
	// state (env vars, dotfiles, local configs). These execute FIRST
	// so that connection strings, feature flags, and runtime config
	// are available before any database driver attempts to connect.
	PriorityEnvironment DriverPriority = 100

	// PriorityDatabase is for drivers that manage persistent data
	// stores (SQLite, PostgreSQL, Redis). These execute AFTER
	// environment drivers, ensuring that restored config is in
	// place before database operations begin.
	PriorityDatabase DriverPriority = 200
)

// CaptureMetadata holds driver-specific informational metadata.
// This is stored in the manifest but never used for restore logic.
type CaptureMetadata map[string]interface{}

// StateDriver is the core abstraction for capturing and restoring
// external system state. Each driver encapsulates the logic for
// one state backend (SQLite, PostgreSQL, Redis, .env files, etc.).
//
// Implementations must be safe for sequential use. The Orchestrator
// guarantees that Capture and Restore are never called concurrently
// on the same driver instance.
type StateDriver interface {
	// Name returns the unique identifier for this driver.
	// Must be lowercase, alphanumeric, no spaces. Example: "sqlite"
	Name() string

	// Priority returns the execution order for this driver.
	// The Orchestrator sorts drivers by ascending priority value
	// before executing Capture or Restore. Use the PriorityEnvironment
	// and PriorityDatabase constants.
	Priority() DriverPriority

	// Capture serializes the current state of the source into a
	// binary stream. The caller owns the returned Reader and is
	// responsible for closing it.
	//
	// The source parameter is the driver-specific locator from
	// config (e.g., a file path or connection URI).
	Capture(ctx context.Context, source string) (io.ReadCloser, CaptureMetadata, error)

	// Restore deserializes a binary stream back into the target
	// system state. The caller provides the Reader obtained from
	// the CAS.
	//
	// Restore must be idempotent: calling it twice with the same
	// blob must produce the same system state.
	Restore(ctx context.Context, source string, blob io.Reader) error

	// Verify checks whether the current live state matches the
	// given blob hash without performing a full restore. This
	// enables fast drift detection.
	Verify(ctx context.Context, source string, expectedHash string) (bool, error)
}
