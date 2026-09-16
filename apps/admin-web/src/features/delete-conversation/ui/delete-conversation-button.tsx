import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { toast } from "sonner";

import { conversationKeys } from "@/entities/conversation";
import { apiClient } from "@/shared/api";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger, Button } from "@/shared/ui";

type DeleteConversationButtonProps = {
  conversationId: string;
  onDeleted?: (() => void) | undefined;
  ariaLabel?: string | undefined;
  compact?: boolean | undefined;
  disabled?: boolean | undefined;
};

export function DeleteConversationButton({ conversationId, onDeleted, ariaLabel = "Delete conversation", compact = false, disabled = false }: DeleteConversationButtonProps) {
  const queryClient = useQueryClient();
  const mutation = useMutation({
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
  return <AlertDialog><AlertDialogTrigger asChild><Button type="button" variant="ghost" size={compact ? "icon" : "sm"} disabled={disabled || mutation.isPending} aria-label={ariaLabel} title={ariaLabel} onClick={(event) => event.stopPropagation()}><Trash2 /></Button></AlertDialogTrigger><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Delete this conversation?</AlertDialogTitle><AlertDialogDescription>It will no longer appear in conversation history. Related activity records will remain available for review.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Keep it</AlertDialogCancel><AlertDialogAction disabled={mutation.isPending} onClick={(event) => { event.stopPropagation(); mutation.mutate(); }}>Delete conversation</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>;
}
