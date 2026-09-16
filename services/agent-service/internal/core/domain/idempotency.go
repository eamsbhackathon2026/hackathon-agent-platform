package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

// IdempotencyRecord binds one caller key to the normalized request and run.
type IdempotencyRecord struct {
	PrincipalKind  PrincipalKind
	PrincipalID    uuid.UUID
	Key            string
	RequestHash    []byte
	RunID          uuid.UUID
	ResponseStatus int
}

// IdempotencyRequestHash returns a stable digest for method, path and JSON body.
func IdempotencyRequestHash(method, path string, body []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, errors.New("invalid idempotency JSON body")
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append(append([]byte(method+"\n"+path+"\n"), canonical...), '\n'))
	return digest[:], nil
}
