package domain

import (
	"time"

	"github.com/google/uuid"
)

// RunJob is a durable queue entry for one asynchronous run.
type RunJob struct {
	RunID       uuid.UUID
	Attempts    int
	AvailableAt time.Time
}
