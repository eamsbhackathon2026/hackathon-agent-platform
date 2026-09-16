import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router";

import { agentQueries } from "@/entities/agent";
import { conversationQueries, type ConversationFilters } from "@/entities/conversation";
import { DeleteConversationButton } from "@/features/delete-conversation";
import { formatDate, problemToAction } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Button, Card, CardContent, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui";

export function ConversationsPage() {
  const [filters, setFilters] = useState<ConversationFilters>({});
  const conversations = useInfiniteQuery(conversationQueries.infiniteList(filters));
  const agents = useQuery(agentQueries.list());
  const names = new Map(agents.data?.map((agent) => [agent.id, agent.name]));
  const items = conversations.data?.pages.flatMap((page) => page.items) ?? [];
  const errorAction = problemToAction((conversations.error as { code?: string })?.code);
  return <div className="space-y-5"><div><h1 className="text-2xl font-semibold">Conversation History</h1><p className="text-sm text-muted-foreground">Review conversations from the admin portal and external systems.</p></div>
    <div className="grid gap-3 md:grid-cols-2"><Select value={filters.agentId ?? "all"} onValueChange={(value) => setFilters({ ...filters, agentId: value === "all" ? undefined : value })}><SelectTrigger><SelectValue placeholder="All assistants" /></SelectTrigger><SelectContent><SelectItem value="all">All assistants</SelectItem>{agents.data?.map((agent) => <SelectItem key={agent.id} value={agent.id}>{agent.name}</SelectItem>)}</SelectContent></Select>
      <Select value={filters.source ?? "all"} onValueChange={(value) => setFilters({ ...filters, source: value === "all" ? undefined : value as ConversationFilters["source"] })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">All sources</SelectItem><SelectItem value="playground">Admin portal</SelectItem><SelectItem value="api">External system</SelectItem></SelectContent></Select></div>
    <Card><CardContent className="pt-6">{conversations.isPending ? <p className="py-12 text-center text-sm text-muted-foreground">Loading conversation history…</p> : conversations.isError ? <Alert variant="destructive"><AlertTitle>{errorAction.title}</AlertTitle><AlertDescription>{errorAction.description} <button className="font-medium underline" onClick={() => void conversations.refetch()}>{errorAction.action?.label ?? "Try again"}</button></AlertDescription></Alert> : items.length ? <Table><TableHeader><TableRow><TableHead>Title</TableHead><TableHead>Assistant</TableHead><TableHead>Source</TableHead><TableHead>Updated</TableHead><TableHead /></TableRow></TableHeader><TableBody>{items.map((item) => <TableRow key={item.id}><TableCell><Link className="font-medium hover:underline" to={`/conversations/${item.id}`}>{item.title || "Untitled conversation"}</Link></TableCell><TableCell>{names.get(item.agent_id) ?? "Archived assistant"}</TableCell><TableCell>{item.source === "playground" ? "Admin portal" : "External system"}</TableCell><TableCell>{formatDate(item.updated_at)}</TableCell><TableCell><DeleteConversationButton conversationId={item.id} /></TableCell></TableRow>)}</TableBody></Table> : <p className="py-12 text-center text-sm text-muted-foreground">No conversations match these filters. Start a conversation with an assistant.</p>}</CardContent></Card>
    {conversations.hasNextPage ? <Button variant="outline" disabled={conversations.isFetchingNextPage} onClick={() => void conversations.fetchNextPage()}>Load more</Button> : null}
  </div>;
}
