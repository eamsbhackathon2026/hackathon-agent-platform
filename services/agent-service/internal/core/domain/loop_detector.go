package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
)

// LoopDetector stops an identical tool call and result after the configured count.
type LoopDetector struct {
	threshold int
	seen      map[[sha256.Size]byte]int
	results   map[[sha256.Size]byte]int
}

// LoopObservation distinguishes an early warning from a no-progress loop that
// must stop. SameResultCount also catches callers that keep changing arguments
// while receiving the same output.
type LoopObservation struct {
	RepetitionCount int
	SameResultCount int
	Warning         bool
	Critical        bool
}

const (
	sameResultWarningThreshold  = 4
	sameResultCriticalThreshold = 6
)

// NewLoopDetector creates a detector; thresholds below one are rejected.
func NewLoopDetector(threshold int) (*LoopDetector, error) {
	if threshold < 1 {
		return nil, ErrValidation
	}
	return &LoopDetector{threshold: threshold, seen: make(map[[sha256.Size]byte]int), results: make(map[[sha256.Size]byte]int)}, nil
}

// Observe canonicalizes JSON object key order before fingerprinting.
func (d *LoopDetector) Observe(call ToolCall, result ToolResult) (bool, error) {
	observation, err := d.ObserveProgress(call, result)
	return observation.Critical, err
}

// ObserveProgress returns warning and critical signals for one completed tool
// call while preserving Observe's compatibility for simpler callers.
func (d *LoopDetector) ObserveProgress(call ToolCall, result ToolResult) (LoopObservation, error) {
	if !json.Valid(call.Arguments) {
		return LoopObservation{}, ErrValidation
	}
	decoder := json.NewDecoder(bytes.NewReader(call.Arguments))
	decoder.UseNumber()
	var args any
	if err := decoder.Decode(&args); err != nil {
		return LoopObservation{}, ErrValidation
	}
	canonical, err := json.Marshal(args)
	if err != nil {
		return LoopObservation{}, ErrValidation
	}
	payload := make([]byte, 0, len(call.Name)+len(canonical)+len(result.Content)+4)
	payload = append(payload, call.Name...)
	payload = append(payload, 0)
	payload = append(payload, canonical...)
	payload = append(payload, 0)
	payload = append(payload, result.Content...)
	payload = append(payload, boolByte(result.IsError), boolByte(result.Truncated))
	fingerprint := sha256.Sum256(payload)
	d.seen[fingerprint]++

	resultPayload := make([]byte, 0, len(call.Name)+len(result.Content)+3)
	resultPayload = append(resultPayload, call.Name...)
	resultPayload = append(resultPayload, 0)
	resultPayload = append(resultPayload, result.Content...)
	resultPayload = append(resultPayload, boolByte(result.IsError), boolByte(result.Truncated))
	resultFingerprint := sha256.Sum256(resultPayload)
	d.results[resultFingerprint]++

	repetitionCount := d.seen[fingerprint]
	sameResultCount := d.results[resultFingerprint]
	critical := repetitionCount >= d.threshold || sameResultCount >= sameResultCriticalThreshold
	warning := !critical && ((d.threshold > 1 && repetitionCount == d.threshold-1) || sameResultCount == sameResultWarningThreshold)
	return LoopObservation{RepetitionCount: repetitionCount, SameResultCount: sameResultCount, Warning: warning, Critical: critical}, nil
}

func boolByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}
