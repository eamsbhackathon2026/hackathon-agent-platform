// Package runs implements assistant execution and conversation queries.
package runs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

const (
	defaultContextPageSize = 100
	maxMetadataBytes       = 16 * 1024
	maxToolResultBytes     = 16 * 1024
	idempotencyTTL         = 24 * time.Hour
)

// Dependencies supplies all execution boundaries without leaking adapters into core.
type Dependencies struct {
	Agents          outbound.AgentRepository
	Providers       outbound.ProviderRepository
	Sessions        outbound.SessionRepository
	Messages        outbound.MessageRepository
	Snapshots       outbound.ContextSnapshotRepository
	Runs            outbound.RunRepository
	Spans           outbound.SpanRepository
	Tx              outbound.TxManager
	Cipher          outbound.SecretCipher
	Factory         outbound.LLMClientFactory
	Tools           outbound.ToolResolver
	Skills          outbound.SkillResolver
	Queue           outbound.RunQueue
	Idempotency     outbound.IdempotencyStore
	Deliveries      outbound.WebhookDeliveryRepository
	WebhookURLs     outbound.URLValidator
	Clock           outbound.Clock
	IDs             outbound.IDGenerator
	Logger          *slog.Logger
	ContextPageSize int
	CancelPoll      time.Duration
}

// Service owns run execution and its read models.
type Service struct{ Dependencies }

// NewService fails closed when any required port is absent.
func NewService(d Dependencies) (*Service, error) {
	if d.Agents == nil || d.Providers == nil || d.Sessions == nil || d.Messages == nil || d.Snapshots == nil || d.Runs == nil || d.Spans == nil || d.Tx == nil || d.Cipher == nil || d.Factory == nil || d.Tools == nil || d.Skills == nil || d.Clock == nil || d.IDs == nil {
		return nil, errors.New("run dependencies are required")
	}
	if d.ContextPageSize <= 0 || d.ContextPageSize > defaultContextPageSize {
		d.ContextPageSize = defaultContextPageSize
	} else if d.ContextPageSize <= maxToolCallsPerRun {
		d.ContextPageSize = maxToolCallsPerRun + 1
	}
	if d.CancelPoll <= 0 {
		d.CancelPoll = 2 * time.Second
	}
	return &Service{Dependencies: d}, nil
}

func (s *Service) validateCommand(ctx context.Context, p domain.Principal, c inbound.RunCommand) (domain.Agent, domain.Provider, error) {
	if err := domain.Authorize(p, domain.ActionRunsWrite, nil); err != nil {
		return domain.Agent{}, domain.Provider{}, err
	}
	if c.AgentID == uuid.Nil || utf8.RuneCountInString(c.Input) < 1 || utf8.RuneCountInString(c.Input) > 100000 {
		return domain.Agent{}, domain.Provider{}, domain.Invalid("input.message", "Tin nhắn phải có từ 1 đến 100.000 ký tự.")
	}
	if c.SessionID != nil && c.SessionKey != nil {
		return domain.Agent{}, domain.Provider{}, domain.Invalid("session_id", "Chỉ dùng một trong session_id hoặc session_key.")
	}
	if c.SessionKey != nil && (p.Kind != domain.PrincipalAPIKey || *c.SessionKey == "" || utf8.RuneCountInString(*c.SessionKey) > 255) {
		return domain.Agent{}, domain.Provider{}, domain.Invalid("session_key", "Khóa hội thoại chỉ dành cho khóa truy cập và dài tối đa 255 ký tự.")
	}
	if c.Mode != domain.RunModeSync && c.Mode != domain.RunModeStream && c.Mode != domain.RunModeAsync {
		return domain.Agent{}, domain.Provider{}, domain.Invalid("mode", "Chế độ xử lý không hợp lệ.")
	}
	if c.IdempotencyKey != nil && (utf8.RuneCountInString(*c.IdempotencyKey) < 1 || utf8.RuneCountInString(*c.IdempotencyKey) > 255 || len(c.RequestHash) != 32) {
		return domain.Agent{}, domain.Provider{}, domain.Invalid("Idempotency-Key", "Khóa chống lặp phải có từ 1 đến 255 ký tự.")
	}
	if c.WebhookURL != nil {
		if c.Mode != domain.RunModeAsync {
			return domain.Agent{}, domain.Provider{}, domain.Invalid("webhook_url", "Địa chỉ nhận kết quả chỉ dùng cho xử lý nền.")
		}
		if p.Kind != domain.PrincipalAPIKey {
			return domain.Agent{}, domain.Provider{}, domain.Invalid("webhook_url", "Địa chỉ nhận kết quả chỉ dùng với khóa truy cập.")
		}
		if s.WebhookURLs == nil || s.WebhookURLs.ValidateURL(*c.WebhookURL) != nil {
			return domain.Agent{}, domain.Provider{}, domain.Invalid("webhook_url", "Địa chỉ nhận kết quả không được phép truy cập.")
		}
	}
	if c.Mode == domain.RunModeAsync && (s.Queue == nil || s.Idempotency == nil || s.Deliveries == nil) {
		return domain.Agent{}, domain.Provider{}, domain.ErrNotImplemented
	}
	if err := validateMetadata(c.Metadata); err != nil {
		return domain.Agent{}, domain.Provider{}, err
	}
	a, err := s.Agents.GetAgent(ctx, c.AgentID)
	if err != nil {
		return domain.Agent{}, domain.Provider{}, err
	}
	provider, err := s.Providers.GetProvider(ctx, a.ProviderID)
	if err != nil {
		return domain.Agent{}, domain.Provider{}, err
	}
	if err = s.validateSessionReference(ctx, p, c); err != nil {
		return domain.Agent{}, domain.Provider{}, err
	}
	return a, provider, nil
}

func validateMetadata(metadata json.RawMessage) error {
	if len(metadata) == 0 {
		return nil
	}
	if len(metadata) > maxMetadataBytes || !json.Valid(metadata) {
		return domain.Invalid("metadata", "Metadata phải là object JSON không quá 16 KiB.")
	}
	var object map[string]any
	if err := json.Unmarshal(metadata, &object); err != nil || object == nil || len(object) > 50 {
		return domain.Invalid("metadata", "Metadata phải là object JSON có tối đa 50 thuộc tính.")
	}
	return nil
}

func (s *Service) validateSessionReference(ctx context.Context, p domain.Principal, c inbound.RunCommand) error {
	var session domain.Session
	var err error
	if c.SessionID != nil {
		session, err = s.Sessions.GetSession(ctx, *c.SessionID)
	} else if c.SessionKey != nil {
		session, err = s.Sessions.GetSessionByExternalKey(ctx, p.APIKeyID, *c.SessionKey)
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
	} else {
		return nil
	}
	if err != nil {
		return err
	}
	return validateContinuationOwner(p, c.AgentID, session)
}

func validateContinuationOwner(p domain.Principal, agentID uuid.UUID, session domain.Session) error {
	if session.AgentID != agentID {
		return domain.ErrNotFound
	}
	if p.Kind == domain.PrincipalUser && session.CreatedByUserID != nil && *session.CreatedByUserID == p.UserID {
		return nil
	}
	if p.Kind == domain.PrincipalAPIKey && session.CreatedByAPIKeyID != nil && *session.CreatedByAPIKeyID == p.APIKeyID {
		return nil
	}
	return domain.ErrNotFound
}

func (s *Service) client(ctx context.Context, provider domain.Provider) (outbound.LLMClient, error) {
	if len(provider.APIKeyCiphertext) == 0 {
		return nil, domain.ErrProviderNotConfigured
	}
	key, err := s.Cipher.Decrypt(provider.APIKeyCiphertext, []byte("provider:"+provider.ID.String()))
	if err != nil {
		return nil, domain.ErrProviderNotConfigured
	}
	connection := domain.ProviderConnection{Kind: provider.Kind, APIKey: string(key)}
	if provider.BaseURL != nil {
		connection.BaseURL = *provider.BaseURL
	}
	return s.Factory.New(ctx, connection)
}

func runSource(p domain.Principal) domain.RunSource {
	if p.Kind == domain.PrincipalAPIKey {
		return domain.RunSourceAPI
	}
	return domain.RunSourcePlayground
}

func sessionTitle(input string) string {
	title := firstSpokenLine(input)
	if title == "" {
		return "Cuộc hội thoại mới"
	}
	trimmed, cut := domain.TruncateUTF8(title, 80)
	if cut {
		trimmed = trimTrailingPartialWord(trimmed)
	}
	return trimmed
}

// firstSpokenLine skips the lines a caller prepends for the model rather than for a
// reader. An integration that carries session context does it as a bracketed line of
// key-value pairs above the question — that line names the conversation no better than
// a header names a letter, and it leaks machine detail into a list people read.
//
// The key-value shape is what makes this safe to apply to every caller: a bracketed
// line a person actually wrote ("[Báo cáo tháng 9]") keeps its place as the title.
func firstSpokenLine(input string) string {
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isMachineContextLine(line) {
			continue
		}
		return line
	}
	return ""
}

func isMachineContextLine(line string) bool {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return false
	}
	return strings.ContainsAny(line, ":=")
}

// trimTrailingPartialWord drops the word the byte limit cut in half, so a title ends
// on something readable instead of mid-syllable. A title with no space to fall back
// on keeps its cut form: half a long word still beats an empty title.
func trimTrailingPartialWord(title string) string {
	space := strings.LastIndexByte(title, ' ')
	if space <= 0 {
		return title
	}
	return strings.TrimRight(title[:space], " ")
}

var _ inbound.RunUseCase = (*Service)(nil)
var _ inbound.SessionQueryUseCase = (*Service)(nil)
