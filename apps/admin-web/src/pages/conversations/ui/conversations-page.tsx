import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { ListFilter } from "lucide-react";
import { useState } from "react";

import { agentQueries } from "@/entities/agent";
import { conversationQueries, type ConversationFilters } from "@/entities/conversation";
import { problemToAction } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Button, Card, CardContent, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Skeleton, Table, TableBody, TableCard, TableCardState, TableHead, TableHeader, TableRow } from "@/shared/ui";
import { ConversationRow } from "./conversation-row";

// The list leads with the conversations people touched most recently, which is what
// the "Updated" column shows and what the time filters apply to.
const initialFilters: ConversationFilters = { sort: "updated_at" };

export function ConversationsPage() {
  const [filters, setFilters] = useState<ConversationFilters>(initialFilters);
  // Remounting the date inputs is the only way to clear them, since they are uncontrolled.
  const [resetKey, setResetKey] = useState(0);
  const invalidWindow = Boolean(filters.from && filters.to && filters.from >= filters.to);
  const conversations = useInfiniteQuery({ ...conversationQueries.infiniteList(filters), enabled: !invalidWindow });
  const agents = useQuery(agentQueries.list());
  const names = new Map(agents.data?.map((agent) => [agent.id, agent.name]));
  const items = conversations.data?.pages.flatMap((page) => page.items) ?? [];
  const errorAction = problemToAction((conversations.error as { code?: string })?.code);
  const hasFilters = Boolean(filters.agentId || filters.source || filters.from || filters.to);
  const updateFilters = (next: Partial<ConversationFilters>) => setFilters((current) => ({ ...current, ...next }));
  const clearFilters = () => { setFilters(initialFilters); setResetKey((key) => key + 1); };
  const summary = items.length ? `Showing ${items.length} ${items.length === 1 ? "conversation" : "conversations"}, most recently updated first` : "Conversations with your assistants";

  return <div className="space-y-5">
    <Card>
      <CardContent className="pt-5 sm:pt-6">
        <div className="mb-4 flex items-center gap-2 text-sm font-semibold"><ListFilter className="size-4" />Filter conversations</div>
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <div className="space-y-2"><Label htmlFor="conversation-agent">Assistant</Label><Select value={filters.agentId ?? "all"} onValueChange={(value) => updateFilters({ agentId: value === "all" ? undefined : value })}><SelectTrigger id="conversation-agent"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">All assistants</SelectItem>{agents.data?.map((agent) => <SelectItem key={agent.id} value={agent.id}>{agent.name}</SelectItem>)}</SelectContent></Select></div>
          <div className="space-y-2"><Label htmlFor="conversation-source">Source</Label><Select value={filters.source ?? "all"} onValueChange={(value) => updateFilters({ source: value === "all" ? undefined : value as ConversationFilters["source"] })}><SelectTrigger id="conversation-source"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">All sources</SelectItem><SelectItem value="playground">Admin portal</SelectItem><SelectItem value="api">External system</SelectItem></SelectContent></Select></div>
          <div className="space-y-2"><Label htmlFor="conversation-from">Active from</Label><Input key={`from-${resetKey}`} id="conversation-from" type="datetime-local" onChange={(event) => updateFilters({ from: event.target.value ? new Date(event.target.value).toISOString() : undefined })} /></div>
          <div className="space-y-2"><Label htmlFor="conversation-to">Active until</Label><Input key={`to-${resetKey}`} id="conversation-to" type="datetime-local" onChange={(event) => updateFilters({ to: event.target.value ? new Date(event.target.value).toISOString() : undefined })} /></div>
        </div>
        {invalidWindow ? <p className="mt-3 text-sm text-destructive" role="alert">“Active from” must be earlier than “Active until”. Adjust one of the dates to see conversations.</p> : null}
      </CardContent>
    </Card>

    <TableCard title="Conversation history" description={summary}>
      {invalidWindow ? <TableCardState className="py-12 text-center text-sm text-muted-foreground">Fix the date range above to see conversations.</TableCardState> : conversations.isPending ? <TableCardState className="space-y-3"><Skeleton className="h-11 w-full" /><Skeleton className="h-16 w-full" /><Skeleton className="h-16 w-full" /></TableCardState> : conversations.isError ? <TableCardState><Alert variant="destructive"><AlertTitle>{errorAction.title}</AlertTitle><AlertDescription>{errorAction.description} <button className="font-medium underline" onClick={() => void conversations.refetch()}>{errorAction.action?.label ?? "Try again"}</button></AlertDescription></Alert></TableCardState> : items.length ? <Table>
        <TableHeader><TableRow><TableHead>Conversation</TableHead><TableHead>Latest status</TableHead><TableHead>Turns</TableHead><TableHead>Assistant</TableHead><TableHead className="hidden lg:table-cell">Usage / time</TableHead><TableHead className="hidden xl:table-cell">Source</TableHead><TableHead>Updated</TableHead><TableHead className="text-right"><span className="sr-only">Actions</span></TableHead></TableRow></TableHeader>
        <TableBody>{items.map((item) => <ConversationRow key={item.id} conversation={item} assistantName={names.get(item.agent_id) ?? "Archived assistant"} />)}</TableBody>
      </Table> : <TableCardState className="grid min-h-52 place-items-center text-center"><div><p className="font-medium">No conversations match these filters.</p><p className="mt-1 text-sm text-muted-foreground">{hasFilters ? "Adjust the filters to see more conversations." : "Start a conversation with an assistant in the Playground."}</p>{hasFilters ? <Button className="mt-4" variant="outline" onClick={clearFilters}>Clear filters</Button> : null}</div></TableCardState>}
    </TableCard>
    {conversations.hasNextPage ? <Button variant="outline" disabled={conversations.isFetchingNextPage} onClick={() => void conversations.fetchNextPage()}>{conversations.isFetchingNextPage ? "Loading…" : "Load more"}</Button> : null}
  </div>;
}
