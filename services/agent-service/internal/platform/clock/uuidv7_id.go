// Package clock supplies production time and identifier sources.
package clock

import (
	"time"

	"github.com/google/uuid"
)

// RealClock reads the system wall clock.
type RealClock struct{}

// Now returns the current time in UTC.
func (RealClock) Now() time.Time { return time.Now().UTC() }

// UUIDv7IDGenerator creates time-ordered identifiers with random bits.
type UUIDv7IDGenerator struct{}

// NewID returns a new UUID version 7.
func (UUIDv7IDGenerator) NewID() (uuid.UUID, error) { return uuid.NewV7() }
