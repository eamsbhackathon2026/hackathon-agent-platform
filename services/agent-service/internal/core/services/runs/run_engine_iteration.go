package runs

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

const finalIterationInstruction = "[System] This is the final processing step. Use the available findings to answer the user now. Do not call any more tools."

type executionState struct {
	run              domain.Run
	session          domain.Session
	rootSpanID       uuid.UUID
	history          []domain.ChatMessage
	units            []contextUnit
	snapshot         *domain.ContextSnapshot
	budgeter         contextBudgeter
	promptEstimate   int
	calibrationSaved bool
	compactionFailed bool
	protectedFromSeq int64
	usage            usageAccumulator
	loop             *domain.LoopDetector
	totalToolCalls   int
}

type usageAccumulator struct {
	input, output           int
	inputKnown, outputKnown bool
	calls                   int
}

func newUsageAccumulator() usageAccumulator {
	return usageAccumulator{inputKnown: true, outputKnown: true}
}

func (u *usageAccumulator) add(usage domain.TokenUsage) {
	u.calls++
	if usage.InputTokens == nil {
		u.inputKnown = false
	} else {
		u.input += *usage.InputTokens
	}
	if usage.OutputTokens == nil {
		u.outputKnown = false
	} else {
		u.output += *usage.OutputTokens
	}
}

func (u usageAccumulator) result() domain.TokenUsage {
	result := domain.TokenUsage{}
	if u.calls > 0 && u.inputKnown {
		value := u.input
		result.InputTokens = &value
	}
	if u.calls > 0 && u.outputKnown {
		value := u.output
		result.OutputTokens = &value
	}
	return result
}

func mustLoopDetector() *domain.LoopDetector {
	detector, _ := domain.NewLoopDetector(3)
	return detector
}

func (s *Service) iterate(ctx context.Context, agent domain.Agent, client outbound.LLMClient, tools outbound.ToolSet, state *executionState, emit func(domain.RunEvent)) *domain.RunFailure {
	policy := iterationPolicy{}
	for iteration := 1; iteration <= agent.MaxIterations; iteration++ {
		if ctx.Err() != nil {
			return failureFromCause(context.Cause(ctx))
		}
		state.run.Iterations = iteration
		started := s.Clock.Now()
		spanID, err := s.IDs.NewID()
		if err != nil {
			return internalFailure()
		}
		declaredTools := tools.Specs()
		finalIteration := iteration == agent.MaxIterations
		requestMessages, requestTools, forcedFinal := policy.prepareRequest(state.history, declaredTools, iteration, agent.MaxIterations, state.totalToolCalls)
		extraMessages := cloneChatMessages(requestMessages[len(state.history):])
		var budget contextBudget
		for {
			requestMessages = append(cloneChatMessages(state.history), extraMessages...)
			var compacted bool
			var failure *domain.RunFailure
			requestMessages, budget, compacted, failure = s.prepareContextRequest(ctx, agent, client, requestMessages, requestTools, state)
			if failure != nil {
				return failure
			}
			if !compacted {
				break
			}
		}
		state.promptEstimate = budget.rawEstimate
		state.calibrationSaved = false
		result, callErr := client.Stream(ctx, domain.LLMRequest{Model: agent.Model, SystemPrompt: agent.SystemPrompt, Messages: requestMessages, Tools: requestTools, Temperature: agent.Temperature, MaxOutputTokens: agent.MaxOutputTokens}, func(delta domain.LLMDelta) {
			if delta.Text != "" && ctx.Err() == nil {
				emit(domain.RunEvent{Type: domain.EventMessageDelta, Text: delta.Text})
			}
		})
		ended := s.Clock.Now()
		spanFailure := safeRunFailure(callErr)
		if callErr == nil {
			state.usage.add(result.Usage)
			s.savePromptCalibration(ctx, state, budget.rawEstimate, result.Usage.InputTokens)
		}
		if err = s.recordLLMSpan(ctx, state, spanID, agent.Model, started, ended, result, spanFailure); err != nil {
			return internalFailure()
		}
		if ctx.Err() != nil {
			return failureFromCause(context.Cause(ctx))
		}
		if callErr != nil {
			return spanFailure
		}
		if (finalIteration || forcedFinal) && len(result.ToolCalls) > 0 {
			return &domain.RunFailure{Code: "max_iterations_reached", Message: "Trợ lý đã đạt giới hạn số bước xử lý."}
		}
		if retry, failure := policy.inspectResponse(result, declaredTools, iteration, agent.MaxIterations); failure != nil {
			return failure
		} else if retry {
			continue
		}
		var valid bool
		result.ToolCalls, valid = normalizeToolCallIDs(result.ToolCalls, result.ProviderMeta, state.history, state.run.ID, iteration)
		if !valid {
			return &domain.RunFailure{Code: "validation_failed", Message: "Kết nối AI trả về lời gọi công cụ không hợp lệ."}
		}
		if len(result.ToolCalls) > 0 && state.totalToolCalls+len(result.ToolCalls) > maxToolCallsPerRun {
			policy.forceFinal = true
			policy.preservePartialText(result)
			continue
		}
		message, err := s.appendAssistant(ctx, state.run, result)
		if err != nil {
			return internalFailure()
		}
		assistantChat := domain.ChatMessage{Role: "assistant", Text: result.Text, ToolCalls: append([]domain.ToolCall(nil), result.ToolCalls...), ProviderMeta: append([]byte(nil), result.ProviderMeta...)}
		state.history = append(state.history, assistantChat)
		state.units = append(state.units, contextUnit{messages: []domain.ChatMessage{assistantChat}, throughSeq: message.Seq})
		emit(domain.RunEvent{Type: domain.EventMessageCompleted, Message: &message})
		if len(result.ToolCalls) == 0 {
			output := result.Text
			state.run.Output = &output
			return nil
		}
		for _, call := range result.ToolCalls {
			failure, warning := s.executeTool(ctx, state, tools, call, emit)
			if failure != nil {
				return failure
			}
			policy.scheduleInstruction(warning)
		}
	}
	return &domain.RunFailure{Code: "max_iterations_reached", Message: "Trợ lý đã đạt giới hạn số bước xử lý."}
}

func (s *Service) executeTool(ctx context.Context, state *executionState, tools outbound.ToolSet, call domain.ToolCall, emit func(domain.RunEvent)) (*domain.RunFailure, string) {
	if ctx.Err() != nil {
		return failureFromCause(context.Cause(ctx)), ""
	}
	emit(domain.RunEvent{Type: domain.EventToolStarted, CallID: call.ID, ToolName: call.Name, DisplayName: domain.ToolDisplayName(call.Name)})
	if ctx.Err() != nil {
		return failureFromCause(context.Cause(ctx)), ""
	}
	started := s.Clock.Now()
	spanID, err := s.IDs.NewID()
	if err != nil {
		return internalFailure(), ""
	}
	state.totalToolCalls++
	result := tools.Execute(ctx, call)
	result.CallID = call.ID
	result.Name = call.Name
	loop, loopErr := state.loop.ObserveProgress(call, result)
	truncated := result.Truncated
	var truncatedAtRunBoundary bool
	result.Content, truncatedAtRunBoundary = domain.TruncateUTF8(result.Content, maxToolResultBytes)
	truncated = truncated || truncatedAtRunBoundary
	ended := s.Clock.Now()
	message, err := s.appendTool(ctx, state.run, result)
	if err != nil {
		return internalFailure(), ""
	}
	state.history = appendToolResult(state.history, result)
	if len(state.units) > 0 {
		last := &state.units[len(state.units)-1]
		last.messages = appendToolResult(last.messages, result)
		last.throughSeq = message.Seq
	}
	if err = s.recordToolSpan(ctx, state, spanID, call, result, truncated, started, ended); err != nil {
		return internalFailure(), ""
	}
	emit(domain.RunEvent{Type: domain.EventToolFinished, CallID: call.ID, OK: !result.IsError, DurationMS: durationMS(started, ended)})
	if loopErr != nil {
		return &domain.RunFailure{Code: "validation_failed", Message: "Kết nối AI trả về lời gọi công cụ không hợp lệ."}, ""
	}
	if loop.Critical {
		return &domain.RunFailure{Code: "loop_detected", Message: "Trợ lý lặp lại thao tác không tạo thêm kết quả."}, ""
	}
	return nil, loopWarningInstruction(call, loop)
}

func (s *Service) savePromptCalibration(ctx context.Context, state *executionState, estimated int, actual *int) {
	if actual == nil || estimated < 1 || *actual < 0 {
		return
	}
	state.budgeter.observe(estimated, *actual)
	state.session.LastPromptEstimatedTokens = &estimated
	value := *actual
	state.session.LastPromptTokens = &value
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.Sessions.UpdatePromptTokenCalibration(persistCtx, state.run.SessionID, estimated, *actual, s.Clock.Now()); err != nil {
		return
	}
	state.calibrationSaved = true
}

func appendToolResult(history []domain.ChatMessage, result domain.ToolResult) []domain.ChatMessage {
	if len(history) > 0 && history[len(history)-1].Role == "tool" {
		history[len(history)-1].ToolResults = append(history[len(history)-1].ToolResults, result)
		return history
	}
	return append(history, domain.ChatMessage{Role: "tool", ToolResults: []domain.ToolResult{result}})
}

func (s *Service) appendAssistant(ctx context.Context, run domain.Run, result domain.LLMResult) (domain.Message, error) {
	id, err := s.IDs.NewID()
	if err != nil {
		return domain.Message{}, err
	}
	runID := run.ID
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.Messages.AppendMessage(persistCtx, domain.Message{ID: id, SessionID: run.SessionID, RunID: &runID, Role: "assistant", Content: result.Text, ToolCalls: result.ToolCalls, ProviderMeta: result.ProviderMeta, CreatedAt: s.Clock.Now()})
}

func (s *Service) appendTool(ctx context.Context, run domain.Run, result domain.ToolResult) (domain.Message, error) {
	id, err := s.IDs.NewID()
	if err != nil {
		return domain.Message{}, err
	}
	runID, callID, name := run.ID, result.CallID, result.Name
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.Messages.AppendMessage(persistCtx, domain.Message{ID: id, SessionID: run.SessionID, RunID: &runID, Role: "tool", Content: result.Content, ToolCalls: []domain.ToolCall{}, ToolCallID: &callID, ToolName: &name, IsError: result.IsError, CreatedAt: s.Clock.Now()})
}
