package clock

import (
	"testing"
	"time"
)

func TestRealClockAndUUIDv7(t *testing.T) {
	before := time.Now()
	now := (RealClock{}).Now()
	if now.Before(before) || now.After(time.Now()) || now.Location() != time.UTC {
		t.Fatal("invalid real clock")
	}
	generator := UUIDv7IDGenerator{}
	a, err := generator.NewID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := generator.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if a.Version() != 7 || b.Version() != 7 || a == b {
		t.Fatal("invalid UUIDv7")
	}
}
