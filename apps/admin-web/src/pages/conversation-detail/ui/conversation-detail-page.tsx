import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { ArrowLeft, MessageSquare } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router";

import { conversationQueries } from "@/entities/conversation";
import { DeleteConversationButton } from "@/features/delete-conversation";
import { formatDate } from "@/shared/lib";
import { useAuthSession } from "@/shared/api";
import { problemToAction } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Button, Card, CardContent } from "@/shared/ui";
import { ChatThread } from "@/widgets/chat-thread";

export function ConversationDetailPage() {
  const { sessionId = "" } = useParams();
  const navigate = useNavigate();
  const conversation = useQuery(conversationQueries.detail(sessionId));
  const messages = useInfiniteQuery(conversationQueries.messagesInfinite(sessionId));
  const { user } = useAuthSession();
  const canContinue = conversation.data?.source === "playground" && conversation.data.created_by_user_id === user?.id;
  if (conversation.isPending) return <p className="text-sm text-muted-foreground">Loading conversation…</p>;
  if (conversation.isError || !conversation.data) {
    const action = problemToAction((conversation.error as { code?: string })?.code);
    return <Alert variant="destructive"><AlertTitle>{action.title}</AlertTitle><AlertDescription>{action.description} <button className="font-medium underline" onClick={() => void conversation.refetch()}>{action.action?.label ?? "Try again"}</button></AlertDescription></Alert>;
  }
  const messageItems = messages.data?.pages.flatMap((page) => page.items) ?? [];
  return <div className="space-y-5"><Link className="inline-flex items-center gap-2 text-sm hover:underline" to="/conversations"><ArrowLeft className="size-4" />Conversation History</Link>
    <div className="flex flex-wrap items-start justify-between gap-3"><div><h1 className="text-2xl font-semibold">{conversation.data.title || "Conversation"}</h1><p className="text-sm text-muted-foreground">Updated {formatDate(conversation.data.updated_at)}</p></div><div className="flex gap-2">{canContinue ? <Button asChild><Link to={`/playground?agent=${conversation.data.agent_id}&session=${conversation.data.id}`}><MessageSquare />Continue conversation</Link></Button> : null}<DeleteConversationButton conversationId={sessionId} onDeleted={() => navigate("/conversations")} /></div></div>
    {!canContinue ? <Card><CardContent className="pt-6 text-sm text-muted-foreground">This conversation came from an external system or belongs to another user, so it cannot be continued here.</CardContent></Card> : null}
    {messages.isPending ? <p className="py-8 text-center text-sm text-muted-foreground">Loading messages…</p> : messages.isError ? <Alert variant="destructive"><AlertTitle>Unable to load messages</AlertTitle><AlertDescription>Reload the conversation messages. <button className="font-medium underline" onClick={() => void messages.refetch()}>Try again</button></AlertDescription></Alert> : <ChatThread messages={messageItems} />}
    {messages.hasNextPage ? <Button variant="outline" disabled={messages.isFetchingNextPage} onClick={() => void messages.fetchNextPage()}>Load more messages</Button> : null}
  </div>;
}
