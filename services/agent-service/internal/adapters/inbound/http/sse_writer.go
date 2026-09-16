package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
)

const sseWriteTimeout = 30 * time.Second

type sseWriter struct {
	w          http.ResponseWriter
	controller *http.ResponseController
	mu         sync.Mutex
	nextID     uint64
}

func newSSEWriter(w http.ResponseWriter) *sseWriter {
	return &sseWriter{w: w, controller: http.NewResponseController(w), nextID: 1}
}

func (s *sseWriter) Emit(ctx context.Context, event domain.RunEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dto, err := eventDTO(event)
	if err != nil {
		return err
	}
	data, err := json.Marshal(dto)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.deadline(); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(s.w, "id: %d\nevent: %s\ndata: %s\n\n", s.nextID, event.Type, data); err != nil {
		return err
	}
	s.nextID++
	return s.flush()
}

func (s *sseWriter) heartbeat(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.deadline(); err != nil {
		return err
	}
	if _, err := fmt.Fprint(s.w, ": ping\n\n"); err != nil {
		return err
	}
	return s.flush()
}

func (s *sseWriter) deadline() error {
	err := s.controller.SetWriteDeadline(time.Now().Add(sseWriteTimeout))
	if errors.Is(err, http.ErrNotSupported) {
		return nil
	}
	return err
}

func (s *sseWriter) flush() error {
	err := s.controller.Flush()
	if errors.Is(err, http.ErrNotSupported) {
		return nil
	}
	return err
}
