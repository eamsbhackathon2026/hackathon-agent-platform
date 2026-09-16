package runs

import "agent-platform/services/agent-service/internal/core/domain"

const missingToolResultContent = "Tool execution did not complete because the previous run was interrupted."

type contextUnit struct {
	messages   []domain.ChatMessage
	throughSeq int64
}

func buildHistory(messages []domain.Message) []domain.ChatMessage {
	return flattenContextUnits(buildContextUnits(messages, make(map[string]bool)))
}

func buildContextUnits(messages []domain.Message, seenCallIDs map[string]bool) []contextUnit {
	units := make([]contextUnit, 0, len(messages))
	for index := 0; index < len(messages); index++ {
		message := messages[index]
		switch message.Role {
		case "user":
			units = append(units, contextUnit{messages: []domain.ChatMessage{{Role: "user", Text: message.Content}}, throughSeq: message.Seq})
		case "assistant":
			copiedCalls, replayIDs, safe := historyToolCallIDs(message, seenCallIDs)
			if !safe {
				unit := contextUnit{throughSeq: message.Seq}
				if message.Content != "" {
					unit.messages = []domain.ChatMessage{{Role: "assistant", Text: message.Content}}
				}
				for index+1 < len(messages) && messages[index+1].Role == "tool" {
					index++
					unit.throughSeq = messages[index].Seq
				}
				units = append(units, unit)
				continue
			}
			unit := contextUnit{messages: []domain.ChatMessage{{Role: "assistant", Text: message.Content, ToolCalls: copiedCalls, ProviderMeta: append([]byte(nil), message.ProviderMeta...)}}, throughSeq: message.Seq}
			if len(copiedCalls) == 0 {
				units = append(units, unit)
				continue
			}

			expected := make(map[string]string, len(copiedCalls))
			for _, call := range copiedCalls {
				expected[call.ID] = call.Name
			}
			results := make([]domain.ToolResult, 0, len(copiedCalls))
			for index+1 < len(messages) && messages[index+1].Role == "tool" {
				index++
				toolMessage := messages[index]
				unit.throughSeq = toolMessage.Seq
				if toolMessage.ToolCallID == nil || toolMessage.ToolName == nil {
					continue
				}
				queue := replayIDs[*toolMessage.ToolCallID]
				if len(queue) == 0 {
					continue
				}
				replayID := queue[0]
				name, found := expected[replayID]
				if !found || name != *toolMessage.ToolName {
					continue
				}
				replayIDs[*toolMessage.ToolCallID] = queue[1:]
				results = append(results, domain.ToolResult{CallID: replayID, Name: *toolMessage.ToolName, Content: toolMessage.Content, IsError: toolMessage.IsError})
				delete(expected, replayID)
			}
			for _, call := range copiedCalls {
				if _, missing := expected[call.ID]; missing {
					results = append(results, domain.ToolResult{CallID: call.ID, Name: call.Name, Content: missingToolResultContent, IsError: true})
				}
			}
			unit.messages = append(unit.messages, domain.ChatMessage{Role: "tool", ToolResults: results})
			units = append(units, unit)
		case "tool":
			// Orphaned or non-contiguous tool results are unsafe to replay.
			units = append(units, contextUnit{throughSeq: message.Seq})
		}
	}
	return units
}

func flattenContextUnits(units []contextUnit) []domain.ChatMessage {
	count := 0
	for _, unit := range units {
		count += len(unit.messages)
	}
	history := make([]domain.ChatMessage, 0, count)
	for _, unit := range units {
		history = append(history, unit.messages...)
	}
	return history
}

func historyToolCallIDs(message domain.Message, seen map[string]bool) ([]domain.ToolCall, map[string][]string, bool) {
	calls := append([]domain.ToolCall(nil), message.ToolCalls...)
	replayIDs := make(map[string][]string, len(calls))
	provisionalSeen := make(map[string]bool, len(seen)+len(calls))
	for id := range seen {
		provisionalSeen[id] = true
	}
	canRewrite := len(message.ProviderMeta) == 0
	for _, call := range calls {
		if len(call.ProviderMeta) > 0 {
			canRewrite = false
			break
		}
	}
	for index := range calls {
		if !validToolCall(calls[index]) {
			return nil, nil, false
		}
		original := calls[index].ID
		if provisionalSeen[original] {
			if !canRewrite {
				return nil, nil, false
			}
			calls[index].ID = uniqueToolCallID(original, message.ID, 0, index, provisionalSeen)
		} else if canRewrite && len(original) > maxReplayToolCallIDLength {
			calls[index].ID = uniqueToolCallID(original, message.ID, 0, index, provisionalSeen)
		}
		provisionalSeen[calls[index].ID] = true
		replayIDs[original] = append(replayIDs[original], calls[index].ID)
	}
	for _, call := range calls {
		seen[call.ID] = true
	}
	return calls, replayIDs, true
}
