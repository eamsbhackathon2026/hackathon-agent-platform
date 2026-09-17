import { Trash2 } from "lucide-react";
import { useRef, useState, type ReactNode } from "react";

import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuLabel, ContextMenuTrigger } from "@/shared/ui";

import { DeleteConversationDialog } from "./delete-conversation-dialog";

type DeleteConversationContextMenuProps = {
  conversationId: string;
  ariaLabel?: string | undefined;
  disabled?: boolean | undefined;
  disabledReason?: string | undefined;
  onDeleted?: (() => void) | undefined;
  children: ReactNode;
};

/**
 * Offers deletion on the wrapped element's own menu: right-click on a pointer,
 * long-press on touch, and the Menu key where the keyboard has one. macOS
 * keyboards do not, and neither Chrome nor Safari raises this menu from
 * Shift+F10 there, so a Mac user without a mouse gesture cannot reach it.
 */
export function DeleteConversationContextMenu({ conversationId, ariaLabel = "Delete conversation", disabled = false, disabledReason, onDeleted, children }: DeleteConversationContextMenuProps) {
  const [open, setOpen] = useState(false);
  const rowRef = useRef<HTMLElement>(null);
  // Radix would hand focus back to the menu item that opened the dialog, but that
  // item is gone by then, so focus lands on the body. Aim it at the row instead
  // whenever the row outlived the dialog — that is, whenever nothing was deleted.
  const restoreFocus = (event: Event) => {
    const row = rowRef.current;
    if (!row?.isConnected) return;
    event.preventDefault();
    row.querySelector("button")?.focus();
  };
  return <>
    <ContextMenu>
      {/*
        The trigger stays enabled even when deleting is not allowed: a disabled
        Radix trigger hands the gesture to the browser's own menu, which lands on
        top of the app saying nothing about why Delete went missing.
      */}
      <ContextMenuTrigger asChild ref={rowRef}>{children}</ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem aria-label={ariaLabel} disabled={disabled} className="text-destructive focus:text-destructive" onSelect={() => setOpen(true)}><Trash2 />Delete</ContextMenuItem>
        {disabled && disabledReason ? <ContextMenuLabel>{disabledReason}</ContextMenuLabel> : null}
      </ContextMenuContent>
    </ContextMenu>
    <DeleteConversationDialog conversationId={conversationId} open={open} onOpenChange={setOpen} onDeleted={onDeleted} onCloseAutoFocus={restoreFocus} />
  </>;
}
