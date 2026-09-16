import { History } from "lucide-react";
import { useState, useSyncExternalStore } from "react";

import { Button, Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from "@/shared/ui";

import { ConversationRailContent, type ConversationTarget } from "./conversation-rail-content";

const desktopQuery = "(min-width: 1280px)";

function getDesktopSnapshot() {
  return typeof window !== "undefined" && typeof window.matchMedia === "function" && window.matchMedia(desktopQuery).matches;
}

function subscribeToDesktopChange(onChange: () => void) {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return () => undefined;
  const media = window.matchMedia(desktopQuery);
  media.addEventListener("change", onChange);
  return () => media.removeEventListener("change", onChange);
}

export type ConversationRailProps = {
  activeSessionId: string;
  disabled: boolean;
  activeDeleteDisabled?: boolean;
  onNew: () => void;
  onSelect: (target: ConversationTarget) => void;
  onActiveDeleted: () => void;
};

function ConversationRailSheet({ activeSessionId, disabled, activeDeleteDisabled = false, onNew, onSelect, onActiveDeleted }: ConversationRailProps) {
  const [open, setOpen] = useState(false);
  const closeAfter = (action: () => void) => () => { action(); setOpen(false); };
  const selectAndClose = (target: ConversationTarget) => { onSelect(target); setOpen(false); };
  const content = <ConversationRailContent activeSessionId={activeSessionId} disabled={disabled} activeDeleteDisabled={activeDeleteDisabled} onNew={closeAfter(onNew)} onSelect={selectAndClose} onActiveDeleted={onActiveDeleted} onNavigate={() => setOpen(false)} />;

  return <Sheet open={open} onOpenChange={setOpen}>
    <SheetTrigger asChild><Button type="button" variant="outline" aria-label="Open recent chats" data-playground-context-trigger="recent"><History />Recent chats</Button></SheetTrigger>
    <SheetContent side="left" className="flex w-[min(22rem,calc(100vw-1.5rem))] flex-col gap-0 overscroll-contain p-0 sm:max-w-[22rem]">
      <SheetHeader className="border-b px-4 py-4 pr-14 text-left"><SheetTitle>Recent conversations</SheetTitle><SheetDescription>Your latest Playground chats</SheetDescription></SheetHeader>
      <div className="flex min-h-0 flex-1 flex-col p-4">{content}</div>
    </SheetContent>
  </Sheet>;
}

export function ConversationRail({ activeDeleteDisabled = false, ...props }: ConversationRailProps) {
  const desktop = useSyncExternalStore(subscribeToDesktopChange, getDesktopSnapshot, () => false);

  if (!desktop) return <ConversationRailSheet {...props} activeDeleteDisabled={activeDeleteDisabled} />;

  return <aside aria-label="Recent conversations" className="flex h-full min-h-[32rem] w-68 shrink-0 flex-col overflow-hidden rounded-[18px] border bg-card shadow-[var(--shadow-card)]">
    <div className="border-b px-4 py-4"><h2 className="font-semibold">Recent conversations</h2><p className="mt-1 text-xs text-muted-foreground">Your latest Playground chats</p></div>
    <div className="flex min-h-0 flex-1 flex-col p-4">
      <ConversationRailContent {...props} activeDeleteDisabled={activeDeleteDisabled} onNavigate={() => undefined} />
    </div>
  </aside>;
}
