package runs

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// executeWithSafetyNet contains panics at the execution boundary after a run is
// durable. It never retries work because an LLM or tool may already have caused
// an external side effect.
func (s *Service) executeWithSafetyNet(runID uuid.UUID, cancel context.CancelCauseFunc, execute func() *domain.RunFailure) (failure *domain.RunFailure) {
	defer func() {
		if recover() == nil {
			return
		}
		s.logExecutionPanic(runID, "execution")
		cancel(errExecutionPanic)
		failure = internalFailure()
	}()
	return execute()
}

// prepareAsyncExecutionSafely protects only dependency setup. Terminalization is
// deliberately performed by the caller outside this boundary so finish cannot
// recursively recover itself.
func (s *Service) prepareAsyncExecutionSafely(runID uuid.UUID, prepare func() *domain.RunFailure) (failure *domain.RunFailure) {
	defer func() {
		if recover() == nil {
			return
		}
		s.logExecutionPanic(runID, "async_setup")
		failure = internalFailure()
	}()
	return prepare()
}

func (s *Service) closeToolSetSafely(runID uuid.UUID, toolset outbound.ToolSet) {
	defer func() {
		if recover() != nil {
			s.logExecutionPanic(runID, "toolset_close")
		}
	}()
	_ = toolset.Close()
}

func (s *Service) logExecutionPanic(runID uuid.UUID, component string) {
	if s.Logger == nil {
		return
	}
	// Panic values, prompts, tool arguments, results and stack locals may contain
	// credentials. Log only stable identifiers needed to correlate the run.
	s.Logger.Error("Agent harness recovered a panic", "run_id", runID, "component", component)
}
