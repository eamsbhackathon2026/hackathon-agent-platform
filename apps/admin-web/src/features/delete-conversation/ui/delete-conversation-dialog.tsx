import type { MouseEvent } from "react";

import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/shared/ui";

import { useDeleteConversation } from "../model/use-delete-conversation";

type DeleteConversationDialogProps = {
  conversationId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted?: (() => void) | undefined;
  onCloseAutoFocus?: ((event: Event) => void) | undefined;
};

/**
 * The single confirmation step for deleting a conversation. Callers own the open
 * state so the dialog can outlive whatever opened it — a context menu item, for
 * instance, unmounts the moment it is chosen.
 */
export function DeleteConversationDialog({ conversationId, open, onOpenChange, onDeleted, onCloseAutoFocus }: DeleteConversationDialogProps) {
  const mutation = useDeleteConversation(conversationId, onDeleted);
  // The confirm button would otherwise close the dialog on the same click that
  // starts the request, leaving the row and its trigger live while the delete is
  // still in flight — long enough on a slow link for someone to ask for it twice.
  // Holding the dialog open until the request settles keeps that window shut.
  const confirm = (event: MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
    if (mutation.isPending) return;
    mutation.mutate(undefined, { onSettled: () => onOpenChange(false) });
  };
  return <AlertDialog open={open} onOpenChange={onOpenChange}><AlertDialogContent {...(onCloseAutoFocus ? { onCloseAutoFocus } : {})}><AlertDialogHeader><AlertDialogTitle>Delete this conversation?</AlertDialogTitle><AlertDialogDescription>It will no longer appear in conversation history. Related activity records will remain available for review.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={mutation.isPending}>Keep it</AlertDialogCancel><AlertDialogAction disabled={mutation.isPending} onClick={confirm}>{mutation.isPending ? "Deleting…" : "Delete conversation"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>;
}
