import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { conversationKeys } from "@/entities/conversation";
import { apiClient } from "@/shared/api";

/** Deletes one conversation and refreshes every list that could still show it. */
export function useDeleteConversation(conversationId: string, onDeleted?: (() => void) | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const { error, response } = await apiClient.DELETE("/v1/sessions/{sessionId}", { params: { path: { sessionId: conversationId } } });
      if (!response.ok) throw error;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: conversationKeys.all });
      toast.success("Conversation deleted");
      onDeleted?.();
    },
    onError: () => toast.error("Unable to delete the conversation"),
  });
}
