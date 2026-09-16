package runs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

type startedRun struct {
	run        domain.Run
	session    domain.Session
	rootSpanID uuid.UUID
	replayed   bool
}

func (s *Service) start(ctx context.Context, p domain.Principal, c inbound.RunCommand) (started startedRun, err error) {
	runID, err := s.IDs.NewID()
	if err != nil {
		return
	}
	err = s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		now := s.Clock.Now()
		if c.IdempotencyKey != nil {
			principalID := p.UserID
			if p.Kind == domain.PrincipalAPIKey {
				principalID = p.APIKeyID
			}
			status := 200
			if c.Mode == domain.RunModeAsync {
				status = 202
			}
			existing, reserveErr := s.Idempotency.Reserve(ctx, domain.IdempotencyRecord{PrincipalKind: p.Kind, PrincipalID: principalID, Key: *c.IdempotencyKey, RequestHash: c.RequestHash, RunID: runID, ResponseStatus: status}, now.Add(idempotencyTTL))
			if reserveErr != nil {
				return reserveErr
			}
			if existing != nil {
				if !bytes.Equal(existing.RequestHash, c.RequestHash) {
					return domain.ErrIdempotencyKeyReused
				}
				started.run, reserveErr = s.Runs.GetRun(ctx, existing.RunID)
				started.replayed = reserveErr == nil
				return reserveErr
			}
		}
		session, resolveErr := s.resolveSession(ctx, p, c)
		if resolveErr != nil {
			return resolveErr
		}
		active, activeErr := s.Sessions.HasActiveRuns(ctx, session.ID)
		if activeErr != nil {
			return activeErr
		}
		if active {
			return &domain.Error{Kind: domain.ErrRunInProgress, Detail: "Hãy chờ lần xử lý hiện tại hoàn tất trước khi gửi tin nhắn tiếp theo."}
		}
		started.session = session
		metadata := c.Metadata
		if len(metadata) == 0 {
			metadata = json.RawMessage(`{}`)
		}
		status := domain.RunRunning
		started.run = domain.Run{ID: runID, AgentID: c.AgentID, SessionID: session.ID, Mode: c.Mode, Status: status, Source: runSource(p), Input: domain.RunInput{Message: c.Input}, Metadata: metadata, WebhookURL: c.WebhookURL, CreatedAt: now}
		if c.Mode == domain.RunModeAsync {
			started.run.Status, started.run.QueuedAt = domain.RunQueued, &now
		} else {
			started.run.StartedAt = &now
		}
		if p.Kind == domain.PrincipalAPIKey {
			started.run.TriggeredByAPIKeyID = &p.APIKeyID
		} else {
			started.run.TriggeredByUserID = &p.UserID
		}
		if createErr := s.Runs.CreateRun(ctx, started.run); createErr != nil {
			return createErr
		}
		messageID, idErr := s.IDs.NewID()
		if idErr != nil {
			return idErr
		}
		runRef := started.run.ID
		if _, appendErr := s.Messages.AppendMessage(ctx, domain.Message{ID: messageID, SessionID: session.ID, RunID: &runRef, Role: "user", Content: c.Input, ToolCalls: []domain.ToolCall{}, CreatedAt: now}); appendErr != nil {
			return appendErr
		}
		if c.Mode == domain.RunModeAsync {
			return s.Queue.Enqueue(ctx, domain.RunJob{RunID: started.run.ID, AvailableAt: now})
		}
		return nil
	})
	if err == nil && !started.replayed && c.Mode != domain.RunModeAsync {
		started.rootSpanID, err = s.IDs.NewID()
	}
	return
}

func (s *Service) resolveSession(ctx context.Context, p domain.Principal, c inbound.RunCommand) (domain.Session, error) {
	if c.SessionID != nil {
		session, err := s.Sessions.GetSessionForUpdate(ctx, *c.SessionID)
		if err != nil {
			return domain.Session{}, err
		}
		return session, validateContinuationOwner(p, c.AgentID, session)
	}
	if c.SessionKey != nil {
		session, err := s.Sessions.GetSessionByExternalKey(ctx, p.APIKeyID, *c.SessionKey)
		if err == nil {
			return session, validateContinuationOwner(p, c.AgentID, session)
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return domain.Session{}, err
		}
	}
	id, err := s.IDs.NewID()
	if err != nil {
		return domain.Session{}, err
	}
	now := s.Clock.Now()
	session := domain.Session{ID: id, AgentID: c.AgentID, Source: runSource(p), ExternalKey: c.SessionKey, Title: sessionTitle(c.Input), CreatedAt: now, UpdatedAt: now}
	if p.Kind == domain.PrincipalAPIKey {
		session.CreatedByAPIKeyID = &p.APIKeyID
	} else {
		session.CreatedByUserID = &p.UserID
	}
	created, err := s.Sessions.CreateSession(ctx, session)
	if err != nil {
		return domain.Session{}, err
	}
	if created || c.SessionKey == nil {
		return session, nil
	}
	existing, err := s.Sessions.GetSessionByExternalKey(ctx, p.APIKeyID, *c.SessionKey)
	if err != nil {
		return domain.Session{}, err
	}
	return existing, validateContinuationOwner(p, c.AgentID, existing)
}
