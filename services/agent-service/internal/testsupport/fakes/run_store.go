package fakes

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// RunStore is an isolated in-memory execution store for core and HTTP tests.
type RunStore struct {
	mu        sync.Mutex
	sessions  map[uuid.UUID]domain.Session
	messages  map[uuid.UUID][]domain.Message
	snapshots map[uuid.UUID][]domain.ContextSnapshot
	runs      map[uuid.UUID]domain.Run
	spans     map[uuid.UUID][]domain.Span
}

// NewRunStore creates empty, isolated run persistence.
func NewRunStore() *RunStore {
	return &RunStore{sessions: map[uuid.UUID]domain.Session{}, messages: map[uuid.UUID][]domain.Message{}, snapshots: map[uuid.UUID][]domain.ContextSnapshot{}, runs: map[uuid.UUID]domain.Run{}, spans: map[uuid.UUID][]domain.Span{}}
}

// WithinTx executes the callback; tests do not inject transactional failures.
func (s *RunStore) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// CreateSession stores a conversation and applies external-key uniqueness.
func (s *RunStore) CreateSession(_ context.Context, value domain.Session) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if value.CreatedByAPIKeyID != nil && value.ExternalKey != nil && session.CreatedByAPIKeyID != nil && session.ExternalKey != nil && *value.CreatedByAPIKeyID == *session.CreatedByAPIKeyID && *value.ExternalKey == *session.ExternalKey {
			return false, nil
		}
	}
	if _, exists := s.sessions[value.ID]; exists {
		return false, domain.ErrConflict
	}
	s.sessions[value.ID] = cloneSession(value)
	return true, nil
}

// GetSession returns a copied conversation.
func (s *RunStore) GetSession(_ context.Context, id uuid.UUID) (domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.sessions[id]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}
	return cloneSession(value), nil
}

// GetSessionForUpdate mirrors GetSession because test operations are serialized.
func (s *RunStore) GetSessionForUpdate(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	return s.GetSession(ctx, id)
}

// GetSessionByExternalKey resolves one API-owned conversation.
func (s *RunStore) GetSessionByExternalKey(_ context.Context, keyID uuid.UUID, external string) (domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, value := range s.sessions {
		if value.CreatedByAPIKeyID != nil && value.ExternalKey != nil && *value.CreatedByAPIKeyID == keyID && *value.ExternalKey == external {
			return cloneSession(value), nil
		}
	}
	return domain.Session{}, domain.ErrNotFound
}

// ListSessions applies the repository filters used by query-service tests.
func (s *RunStore) ListSessions(_ context.Context, options outbound.SessionListOptions) ([]domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Session{}
	for _, value := range s.sessions {
		if options.AgentID != nil && value.AgentID != *options.AgentID || options.Source != nil && value.Source != *options.Source || options.OwnerUserID != nil && (value.CreatedByUserID == nil || *value.CreatedByUserID != *options.OwnerUserID) || options.OwnerAPIKeyID != nil && (value.CreatedByAPIKeyID == nil || *value.CreatedByAPIKeyID != *options.OwnerAPIKeyID) || options.UpdatedFrom != nil && value.UpdatedAt.Before(*options.UpdatedFrom) || options.UpdatedTo != nil && !value.UpdatedAt.Before(*options.UpdatedTo) {
			continue
		}
		if options.OrderByUpdatedAt {
			if options.BeforeUpdated != nil && !beforeUpdatedDescending(value.UpdatedAt, value.ID, *options.BeforeUpdated) {
				continue
			}
		} else if options.Before != nil && !beforeDescending(value.CreatedAt, value.ID, *options.Before) {
			continue
		}
		items = append(items, cloneSession(value))
	}
	sort.Slice(items, func(i, j int) bool {
		if options.OrderByUpdatedAt {
			return newer(items[i].UpdatedAt, items[i].ID, items[j].UpdatedAt, items[j].ID)
		}
		return newer(items[i].CreatedAt, items[i].ID, items[j].CreatedAt, items[j].ID)
	})
	return limitSessions(items, options.Limit), nil
}

// SummarizeSessions counts runs per conversation and previews the first and last
// non-empty user or assistant messages.
func (s *RunStore) SummarizeSessions(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.SessionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[uuid.UUID]domain.SessionSummary, len(ids))
	for _, id := range ids {
		summary := domain.SessionSummary{}
		var latest *domain.Run
		for _, run := range s.runs {
			if run.SessionID != id {
				continue
			}
			summary.TurnCount++
			if run.Status == domain.RunFailed {
				summary.FailedTurnCount++
			}
			if latest == nil || newer(run.CreatedAt, run.ID, latest.CreatedAt, latest.ID) {
				value := run
				latest = &value
			}
		}
		if latest != nil {
			summary.LatestRunID, summary.LatestRunStatus = &latest.ID, &latest.Status
		}
		for _, message := range s.messages[id] {
			if message.Role != "user" && message.Role != "assistant" || strings.TrimSpace(message.Content) == "" {
				continue
			}
			content, role := message.Content, message.Role
			if summary.FirstMessage == nil && role == "user" {
				summary.FirstMessage = &content
			}
			summary.LastMessage, summary.LastMessageRole = &content, &role
		}
		result[id] = summary
	}
	return result, nil
}

// HasActiveRuns reports unfinished executions for a conversation.
func (s *RunStore) HasActiveRuns(_ context.Context, sessionID uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, run := range s.runs {
		if run.SessionID == sessionID && !run.Terminal() {
			return true, nil
		}
	}
	return false, nil
}

// DeleteSession removes a conversation from the in-memory visible set.
func (s *RunStore) DeleteSession(_ context.Context, id uuid.UUID, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.sessions, id)
	delete(s.messages, id)
	delete(s.snapshots, id)
	return nil
}

// UpdatePromptTokenCalibration records the latest successful main request.
func (s *RunStore) UpdatePromptTokenCalibration(_ context.Context, id uuid.UUID, estimated, actual int, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.sessions[id]
	if !ok {
		return domain.ErrNotFound
	}
	if estimated < 1 || actual < 0 {
		return domain.ErrValidation
	}
	value.LastPromptEstimatedTokens = copyPointer(&estimated)
	value.LastPromptTokens = copyPointer(&actual)
	if at.After(value.UpdatedAt) {
		value.UpdatedAt = at
	}
	s.sessions[id] = value
	return nil
}

// AppendMessage assigns the next sequence and stores a copy.
func (s *RunStore) AppendMessage(_ context.Context, value domain.Message) (domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[value.SessionID]; !ok {
		return domain.Message{}, domain.ErrNotFound
	}
	value.Seq = int64(len(s.messages[value.SessionID]) + 1)
	value = cloneMessage(value)
	s.messages[value.SessionID] = append(s.messages[value.SessionID], value)
	session := s.sessions[value.SessionID]
	if value.CreatedAt.After(session.UpdatedAt) {
		session.UpdatedAt = value.CreatedAt
		s.sessions[value.SessionID] = session
	}
	return cloneMessage(value), nil
}

// ListRecentMessages returns a copied history tail.
func (s *RunStore) ListRecentMessages(_ context.Context, sessionID uuid.UUID, limit int) ([]domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.messages[sessionID]
	if len(items) > limit {
		items = items[len(items)-limit:]
	}
	return cloneMessages(items), nil
}

// ListMessages returns an ascending message page.
func (s *RunStore) ListMessages(_ context.Context, sessionID uuid.UUID, limit int, after *outbound.MessageCursor) ([]domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Message{}
	for _, value := range s.messages[sessionID] {
		if after == nil || value.Seq > after.Seq || value.Seq == after.Seq && value.ID.String() > after.ID.String() {
			items = append(items, cloneMessage(value))
		}
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// ListRunMessages returns an ascending message page scoped to one run.
func (s *RunStore) ListRunMessages(_ context.Context, sessionID, runID uuid.UUID, limit int, after *outbound.MessageCursor) ([]domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Message{}
	for _, value := range s.messages[sessionID] {
		if value.RunID != nil && *value.RunID == runID && (after == nil || value.Seq > after.Seq || value.Seq == after.Seq && value.ID.String() > after.ID.String()) {
			items = append(items, cloneMessage(value))
		}
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// ListMessagesAfterSeq returns an ascending internal page after one checkpoint.
func (s *RunStore) ListMessagesAfterSeq(_ context.Context, sessionID uuid.UUID, afterSeq int64, limit int) ([]domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Message{}
	for _, value := range s.messages[sessionID] {
		if value.Seq > afterSeq {
			items = append(items, cloneMessage(value))
		}
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// GetLatestContextSnapshot returns the highest in-memory version.
func (s *RunStore) GetLatestContextSnapshot(_ context.Context, sessionID uuid.UUID) (domain.ContextSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.snapshots[sessionID]
	if len(items) == 0 {
		return domain.ContextSnapshot{}, domain.ErrNotFound
	}
	return cloneContextSnapshot(items[len(items)-1]), nil
}

// CreateContextSnapshotCAS mirrors the database version and coverage guards.
func (s *RunStore) CreateContextSnapshotCAS(_ context.Context, value domain.ContextSnapshot, expectedVersion int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.snapshots[value.SessionID]
	currentVersion, covered := int64(0), int64(0)
	if len(items) > 0 {
		currentVersion = items[len(items)-1].Version
		covered = items[len(items)-1].CoveredThroughSeq
	}
	if currentVersion != expectedVersion || value.Version != expectedVersion+1 || value.CoveredThroughSeq <= covered {
		return false, nil
	}
	s.snapshots[value.SessionID] = append(items, cloneContextSnapshot(value))
	return true, nil
}

// CreateRun stores a copied execution.
func (s *RunStore) CreateRun(_ context.Context, value domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[value.ID]; ok {
		return domain.ErrConflict
	}
	s.runs[value.ID] = cloneRun(value)
	return nil
}

// GetRun returns a copied execution.
func (s *RunStore) GetRun(_ context.Context, id uuid.UUID) (domain.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[id]
	if !ok {
		return domain.Run{}, domain.ErrNotFound
	}
	return cloneRun(value), nil
}

// StartQueuedRun atomically transitions a queued run to running.
func (s *RunStore) StartQueuedRun(_ context.Context, id uuid.UUID, at time.Time) (domain.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[id]
	if !ok || value.Status != domain.RunQueued {
		return domain.Run{}, domain.ErrNotFound
	}
	value.Status, value.StartedAt = domain.RunRunning, &at
	s.runs[id] = value
	return cloneRun(value), nil
}

// ListRuns applies filters and descending ordering.
func (s *RunStore) ListRuns(_ context.Context, options outbound.RunListOptions) ([]domain.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Run{}
	for _, value := range s.runs {
		if options.AgentID != nil && value.AgentID != *options.AgentID || options.SessionID != nil && value.SessionID != *options.SessionID || options.Status != nil && value.Status != *options.Status || options.Source != nil && value.Source != *options.Source || options.From != nil && value.CreatedAt.Before(*options.From) || options.To != nil && !value.CreatedAt.Before(*options.To) || options.OwnerUserID != nil && (value.TriggeredByUserID == nil || *value.TriggeredByUserID != *options.OwnerUserID) || options.OwnerAPIKeyID != nil && (value.TriggeredByAPIKeyID == nil || *value.TriggeredByAPIKeyID != *options.OwnerAPIKeyID) {
			continue
		}
		if options.Before != nil && !beforeDescending(value.CreatedAt, value.ID, *options.Before) {
			continue
		}
		items = append(items, cloneRun(value))
	}
	sort.Slice(items, func(i, j int) bool { return newer(items[i].CreatedAt, items[i].ID, items[j].CreatedAt, items[j].ID) })
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, nil
}

// RequestRunCancel records the first cancellation timestamp.
func (s *RunStore) RequestRunCancel(_ context.Context, id uuid.UUID, at time.Time) (domain.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[id]
	if !ok {
		return domain.Run{}, domain.ErrNotFound
	}
	if !value.Terminal() && value.CancelRequestedAt == nil {
		value.CancelRequestedAt = &at
		s.runs[id] = value
	}
	return cloneRun(value), nil
}

// RunCancelRequested reports the in-memory cancellation flag.
func (s *RunStore) RunCancelRequested(_ context.Context, id uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[id]
	if !ok {
		return false, domain.ErrNotFound
	}
	return value.CancelRequestedAt != nil, nil
}

// FinishRun replaces an active execution with its terminal value.
func (s *RunStore) FinishRun(_ context.Context, value domain.Run) (domain.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.runs[value.ID]
	if !ok || stored.Terminal() {
		return domain.Run{}, domain.ErrNotFound
	}
	s.runs[value.ID] = cloneRun(value)
	return cloneRun(value), nil
}

// CreateSpan appends a trace step.
func (s *RunStore) CreateSpan(_ context.Context, value domain.Span) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spans[value.RunID] = append(s.spans[value.RunID], cloneSpan(value))
	return nil
}

// ListSpans returns an ascending trace page.
func (s *RunStore) ListSpans(_ context.Context, runID uuid.UUID, limit int, after *outbound.SpanCursor) ([]domain.Span, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []domain.Span{}
	for _, value := range s.spans[runID] {
		if after == nil || value.StartedAt.After(after.StartedAt) || value.StartedAt.Equal(after.StartedAt) && value.ID.String() > after.ID.String() {
			items = append(items, cloneSpan(value))
		}
	}
	sort.Slice(items, func(i, j int) bool { return older(items[i].StartedAt, items[i].ID, items[j].StartedAt, items[j].ID) })
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func cloneSession(v domain.Session) domain.Session {
	v.CreatedByUserID = copyPointer(v.CreatedByUserID)
	v.CreatedByAPIKeyID = copyPointer(v.CreatedByAPIKeyID)
	v.ExternalKey = copyPointer(v.ExternalKey)
	v.LastPromptEstimatedTokens = copyPointer(v.LastPromptEstimatedTokens)
	v.LastPromptTokens = copyPointer(v.LastPromptTokens)
	return v
}
func cloneContextSnapshot(v domain.ContextSnapshot) domain.ContextSnapshot {
	v.SourceRunID = copyPointer(v.SourceRunID)
	v.Usage.InputTokens = copyPointer(v.Usage.InputTokens)
	v.Usage.OutputTokens = copyPointer(v.Usage.OutputTokens)
	return v
}
func cloneMessage(v domain.Message) domain.Message {
	v.RunID = copyPointer(v.RunID)
	v.ToolCallID = copyPointer(v.ToolCallID)
	v.ToolName = copyPointer(v.ToolName)
	v.ToolCalls = append([]domain.ToolCall(nil), v.ToolCalls...)
	v.ProviderMeta = append([]byte(nil), v.ProviderMeta...)
	return v
}
func cloneMessages(values []domain.Message) []domain.Message {
	result := make([]domain.Message, len(values))
	for i, value := range values {
		result[i] = cloneMessage(value)
	}
	return result
}
func cloneRun(v domain.Run) domain.Run {
	v.TriggeredByUserID = copyPointer(v.TriggeredByUserID)
	v.TriggeredByAPIKeyID = copyPointer(v.TriggeredByAPIKeyID)
	v.Output = copyPointer(v.Output)
	v.Failure = copyPointer(v.Failure)
	v.Metadata = append([]byte(nil), v.Metadata...)
	v.WebhookURL = copyPointer(v.WebhookURL)
	v.CancelRequestedAt = copyPointer(v.CancelRequestedAt)
	v.QueuedAt = copyPointer(v.QueuedAt)
	v.StartedAt = copyPointer(v.StartedAt)
	v.FinishedAt = copyPointer(v.FinishedAt)
	v.Usage.InputTokens = copyPointer(v.Usage.InputTokens)
	v.Usage.OutputTokens = copyPointer(v.Usage.OutputTokens)
	return v
}
func cloneSpan(v domain.Span) domain.Span {
	v.ParentSpanID = copyPointer(v.ParentSpanID)
	v.Model = copyPointer(v.Model)
	v.ToolName = copyPointer(v.ToolName)
	v.Attributes = append([]byte(nil), v.Attributes...)
	v.ErrorMessage = copyPointer(v.ErrorMessage)
	v.Usage.InputTokens = copyPointer(v.Usage.InputTokens)
	v.Usage.OutputTokens = copyPointer(v.Usage.OutputTokens)
	return v
}
func newer(at time.Time, id uuid.UUID, otherAt time.Time, otherID uuid.UUID) bool {
	return at.After(otherAt) || at.Equal(otherAt) && id.String() > otherID.String()
}
func older(at time.Time, id uuid.UUID, otherAt time.Time, otherID uuid.UUID) bool {
	return at.Before(otherAt) || at.Equal(otherAt) && id.String() < otherID.String()
}
func beforeDescending(at time.Time, id uuid.UUID, cursor domain.PageCursor) bool {
	return at.Before(cursor.CreatedAt) || at.Equal(cursor.CreatedAt) && id.String() < cursor.ID.String()
}
func beforeUpdatedDescending(at time.Time, id uuid.UUID, cursor outbound.SessionUpdatedCursor) bool {
	return at.Before(cursor.UpdatedAt) || at.Equal(cursor.UpdatedAt) && id.String() < cursor.ID.String()
}
func limitSessions(values []domain.Session, limit int) []domain.Session {
	if limit > 0 && len(values) > limit {
		return values[:limit]
	}
	return values
}

var _ outbound.SessionRepository = (*RunStore)(nil)
var _ outbound.MessageRepository = (*RunStore)(nil)
var _ outbound.ContextSnapshotRepository = (*RunStore)(nil)
var _ outbound.RunRepository = (*RunStore)(nil)
var _ outbound.SpanRepository = (*RunStore)(nil)
var _ outbound.TxManager = (*RunStore)(nil)
