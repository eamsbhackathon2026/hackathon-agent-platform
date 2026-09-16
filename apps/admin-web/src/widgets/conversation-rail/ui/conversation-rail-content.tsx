import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { MessageSquarePlus } from "lucide-react";
import { Link } from "react-router";

import { agentQueries } from "@/entities/agent";
import { conversationQueries } from "@/entities/conversation";
import { canManageWorkspace } from "@/entities/session-user";
import { DeleteConversationButton } from "@/features/delete-conversation";
import { formatDate, problemToAction } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Button, ScrollArea, Skeleton } from "@/shared/ui";
import { useAuthSession } from "@/shared/api";
import { cn } from "@/shared/lib/cn";

export type ConversationTarget = { agentId: string; sessionId: string };

type ConversationRailContentProps = {
  activeSessionId: string;
  disabled: boolean;
  activeDeleteDisabled: boolean;
  onNew: () => void;
  onSelect: (target: ConversationTarget) => void;
  onActiveDeleted: () => void;
  onNavigate?: (() => void) | undefined;
};

export function ConversationRailContent({ activeSessionId, disabled, activeDeleteDisabled, onNew, onSelect, onActiveDeleted, onNavigate }: ConversationRailContentProps) {
  const conversations = useInfiniteQuery(conversationQueries.recentMine());
  const agents = useQuery(agentQueries.list());
  const { user } = useAuthSession();
  const items = conversations.data?.pages.flatMap((page) => page.items) ?? [];
  const names = new Map(agents.data?.map((agent) => [agent.id, agent.name]));
  const canViewWorkspace = user ? canManageWorkspace(user.role) : false;
  const loading = conversations.isPending || agents.isPending;
  const failed = conversations.isError || agents.isError;
  const errorAction = problemToAction(((conversations.error ?? agents.error) as { code?: string } | null)?.code);

  let listContent: React.ReactNode;
  if (loading) {
    listContent = <div role="status" aria-label="Loading recent conversations" className="space-y-2 p-1">{Array.from({ length: 5 }, (_, index) => <Skeleton key={index} className="h-[72px] w-full" />)}</div>;
  } else if (failed) {
    listContent = <Alert variant="destructive"><AlertTitle>Unable to load recent conversations</AlertTitle><AlertDescription>{errorAction.description}<Button type="button" variant="outline" size="sm" className="mt-3 w-full" onClick={() => void Promise.all([conversations.refetch(), agents.refetch()])}>{errorAction.action?.label ?? "Try again"}</Button></AlertDescription></Alert>;
  } else if (items.length === 0) {
    listContent = <div className="grid min-h-48 place-items-center px-5 text-center"><div><p className="font-medium">No conversations yet</p><p className="mt-1 text-sm text-muted-foreground">Start a new chat with an assistant.</p></div></div>;
  } else {
    listContent = <ul aria-label="Recent conversations" className="space-y-1 pr-2">{items.map((item) => {
      const title = item.title || "Untitled conversation";
      const assistantName = names.get(item.agent_id) ?? "Archived assistant";
      const updatedLabel = formatDate(item.updated_at);
      const active = item.id === activeSessionId;
      return <li key={item.id} className={cn("flex min-w-0 items-center gap-1 rounded-xl border border-transparent p-1 transition-colors", active ? "border-primary/30 bg-primary/5" : "hover:bg-accent/70")}>
        <button type="button" disabled={disabled} aria-current={active ? "page" : undefined} aria-label={`${title}, ${assistantName}${active ? ", current conversation" : ""}`} className="flex min-h-11 min-w-0 flex-1 flex-col items-start justify-center rounded-lg px-2.5 py-2 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-45" onClick={() => onSelect({ agentId: item.agent_id, sessionId: item.id })}>
          <span className="flex w-full min-w-0 items-center gap-2"><span className="min-w-0 flex-1 truncate text-sm font-medium" title={title}>{title}</span>{active ? <span className="shrink-0 text-[10px] font-semibold uppercase tracking-wide text-primary">Current</span> : null}</span>
          <span className="mt-0.5 flex w-full min-w-0 items-center gap-1.5 text-xs text-muted-foreground"><span className="min-w-0 flex-1 truncate">{assistantName}</span><time className="max-w-32 shrink-0 truncate" dateTime={item.updated_at} title={updatedLabel}>{updatedLabel}</time></span>
        </button>
        <DeleteConversationButton conversationId={item.id} ariaLabel={`Delete ${title}`} compact disabled={disabled || (active && activeDeleteDisabled)} onDeleted={active ? onActiveDeleted : undefined} />
      </li>;
    })}</ul>;
  }

  return <div className="flex min-h-0 flex-1 flex-col gap-3">
    <Button type="button" className="w-full" disabled={disabled} onClick={onNew}><MessageSquarePlus />New chat</Button>
    {disabled ? <p className="rounded-xl border bg-muted/50 px-3 py-2 text-xs text-muted-foreground">Stop or wait for the response before switching conversations.</p> : null}
    <ScrollArea className="min-h-0 flex-1"><div className="pb-2">{listContent}{!loading && !failed && conversations.hasNextPage ? <Button type="button" variant="outline" className="mt-3 w-full" disabled={conversations.isFetchingNextPage} onClick={() => void conversations.fetchNextPage()}>{conversations.isFetchingNextPage ? "Loading more…" : "Load more"}</Button> : null}</div></ScrollArea>
    {canViewWorkspace ? <Link to="/conversations" aria-disabled={disabled || undefined} className={cn("inline-flex min-h-11 items-center justify-center rounded-xl px-3 text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40", disabled && "pointer-events-none opacity-45")} onClick={(event) => { if (disabled) event.preventDefault(); else onNavigate?.(); }}>View workspace conversations</Link> : null}
  </div>;
}
