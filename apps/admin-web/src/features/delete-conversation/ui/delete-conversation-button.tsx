import { Trash2 } from "lucide-react";
import { useState } from "react";

import { Button } from "@/shared/ui";

import { DeleteConversationDialog } from "./delete-conversation-dialog";

type DeleteConversationButtonProps = {
  conversationId: string;
  onDeleted?: (() => void) | undefined;
  ariaLabel?: string | undefined;
  compact?: boolean | undefined;
  disabled?: boolean | undefined;
};

export function DeleteConversationButton({ conversationId, onDeleted, ariaLabel = "Delete conversation", compact = false, disabled = false }: DeleteConversationButtonProps) {
  const [open, setOpen] = useState(false);
  return <>
    <Button type="button" variant="ghost" size={compact ? "icon" : "sm"} disabled={disabled} aria-label={ariaLabel} title={ariaLabel} onClick={(event) => { event.stopPropagation(); setOpen(true); }}><Trash2 /></Button>
    <DeleteConversationDialog conversationId={conversationId} open={open} onOpenChange={setOpen} onDeleted={onDeleted} />
  </>;
}
