package runs

import (
	"context"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

const toolResultPageSize = 100

// attachToolResults derives the run's structured tool results from its transcript.
// Reading the messages back rather than tracking them in memory keeps one source of
// truth for the sync response, the run_completed event, the webhook and GET /runs/{id}.
func (s *Service) attachToolResults(ctx context.Context, run *domain.Run) error {
	messages := make([]domain.Message, 0, toolResultPageSize)
	var cursor *outbound.MessageCursor
	for {
		page, err := s.Messages.ListRunMessages(ctx, run.SessionID, run.ID, toolResultPageSize, cursor)
		if err != nil {
			return err
		}
		messages = append(messages, page...)
		if len(page) < toolResultPageSize {
			break
		}
		last := page[len(page)-1]
		cursor = &outbound.MessageCursor{Seq: last.Seq, ID: last.ID}
	}
	run.ToolResults = domain.ToolResultsFromMessages(messages)
	return nil
}
