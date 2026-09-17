package http

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// RunHandler translates execution and trace operations to the run use case.
type RunHandler struct{ runs inbound.RunUseCase }

// NewRunHandler binds execution operations to their use case.
func NewRunHandler(runs inbound.RunUseCase) *RunHandler { return &RunHandler{runs: runs} }

// CreateRun executes immediately or atomically enqueues an asynchronous run.
func (h *RunHandler) CreateRun(ctx context.Context, request gen.CreateRunRequestObject) (gen.CreateRunResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	command, err := createRunCommand(request.AgentId, request.Body.Input.Message, request.Body.SessionId, request.Body.SessionKey, domain.RunMode(request.Body.Mode), request.Body.Metadata)
	if err != nil {
		return nil, err
	}
	command.WebhookURL = request.Body.WebhookUrl
	if request.Params.IdempotencyKey != nil {
		raw, marshalErr := json.Marshal(request.Body)
		if marshalErr != nil {
			return nil, domain.ErrValidation
		}
		command.IdempotencyKey = request.Params.IdempotencyKey
		command.RequestHash, err = domain.IdempotencyRequestHash("POST", "/v1/agents/"+request.AgentId.String()+"/runs", raw)
		if err != nil {
			return nil, domain.ErrValidation
		}
	}
	principal := requestFrom(ctx).principal
	if request.Body.Mode == gen.RunCreateRequestModeAsync {
		run, enqueueErr := h.runs.EnqueueAsync(ctx, principal, command)
		if enqueueErr != nil {
			return nil, enqueueErr
		}
		location := fmt.Sprintf("/v1/runs/%s", run.ID)
		return gen.CreateRun202JSONResponse{Body: gen.RunAccepted{RunId: run.ID, Status: gen.RunStatus(run.Status)}, Headers: gen.CreateRun202ResponseHeaders{Location: &location}}, nil
	}
	if request.Body.WebhookUrl != nil {
		return nil, domain.Invalid("webhook_url", "Địa chỉ nhận kết quả chỉ dùng cho xử lý nền.")
	}
	run, err := h.runs.RunSync(ctx, principal, command)
	if err != nil {
		return nil, err
	}
	dto, err := runDTO(run)
	return gen.CreateRun200JSONResponse(dto), err
}

// StreamRun validates before returning a response object that owns the live SSE execution.
func (h *RunHandler) StreamRun(ctx context.Context, request gen.StreamRunRequestObject) (gen.StreamRunResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	command, err := createRunCommand(request.AgentId, request.Body.Input.Message, request.Body.SessionId, request.Body.SessionKey, domain.RunModeStream, request.Body.Metadata)
	if err != nil {
		return nil, err
	}
	principal := requestFrom(ctx).principal
	return &runStreamResponse{ctx: ctx, principal: principal, command: command, runs: h.runs}, nil
}

func createRunCommand(agentID gen.AgentId, input string, sessionID *gen.SessionId, sessionKey *string, mode domain.RunMode, metadata *map[string]any) (inbound.RunCommand, error) {
	command := inbound.RunCommand{AgentID: agentID, Input: input, SessionID: sessionID, SessionKey: sessionKey, Mode: mode, Metadata: json.RawMessage(`{}`)}
	if metadata != nil {
		raw, err := json.Marshal(metadata)
		if err != nil {
			return command, domain.ErrValidation
		}
		command.Metadata = raw
	}
	return command, nil
}

// GetRun returns one authorized execution.
func (h *RunHandler) GetRun(ctx context.Context, request gen.GetRunRequestObject) (gen.GetRunResponseObject, error) {
	run, err := h.runs.GetRun(ctx, requestFrom(ctx).principal, request.RunId)
	if err != nil {
		return nil, err
	}
	dto, err := runDTO(run)
	return gen.GetRun200JSONResponse(dto), err
}

// CancelRun records an idempotent stop request.
func (h *RunHandler) CancelRun(ctx context.Context, request gen.CancelRunRequestObject) (gen.CancelRunResponseObject, error) {
	run, err := h.runs.CancelRun(ctx, requestFrom(ctx).principal, request.RunId)
	if err != nil {
		return nil, err
	}
	dto, err := runDTO(run)
	return gen.CancelRun200JSONResponse(dto), err
}

// ListRuns maps public filters and a keyset page.
func (h *RunHandler) ListRuns(ctx context.Context, request gen.ListRunsRequestObject) (gen.ListRunsResponseObject, error) {
	filter := inbound.RunListRequest{PageRequest: pageRequest(request.Params.Limit, request.Params.Cursor), AgentID: request.Params.AgentId, SessionID: request.Params.SessionId, From: request.Params.From, To: request.Params.To}
	if request.Params.Status != nil {
		value := domain.RunStatus(*request.Params.Status)
		filter.Status = &value
	}
	if request.Params.Source != nil {
		value := domain.RunSource(*request.Params.Source)
		filter.Source = &value
	}
	page, err := h.runs.ListRuns(ctx, requestFrom(ctx).principal, filter)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Run, 0, len(page.Items))
	for _, run := range page.Items {
		dto, mapErr := runDTO(run)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, dto)
	}
	return gen.ListRuns200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// ListRunSpans returns one authorized trace page.
func (h *RunHandler) ListRunSpans(ctx context.Context, request gen.ListRunSpansRequestObject) (gen.ListRunSpansResponseObject, error) {
	page, err := h.runs.ListRunSpans(ctx, requestFrom(ctx).principal, request.RunId, pageRequest(request.Params.Limit, request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.Span, 0, len(page.Items))
	for _, span := range page.Items {
		dto, mapErr := spanDTO(span)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, dto)
	}
	return gen.ListRunSpans200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}
