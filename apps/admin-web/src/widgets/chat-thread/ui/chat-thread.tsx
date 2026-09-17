import { Bot, CheckCircle2, LoaderCircle, Wrench, XCircle } from "lucide-react";
import { useEffect, useRef } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import type { ConversationMessage, StreamState } from "@/entities/conversation";
import { Badge, Card, CardContent, Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/shared/ui";

function Markdown({ children }: { children: string }) {
  return <div className="min-w-0 break-words [overflow-wrap:anywhere] [&_pre]:max-w-full [&_pre]:overflow-x-auto">
    <ReactMarkdown skipHtml remarkPlugins={[remarkGfm]} components={{
      a: ({ node, ...props }) => { void node; return <a {...props} className="underline" target="_blank" rel="noopener noreferrer" />; },
      table: ({ node, ...props }) => { void node; return <div className="my-2 max-w-full overflow-x-auto"><table {...props} className="w-full border-collapse text-left" /></div>; },
      th: ({ node, ...props }) => { void node; return <th {...props} className="border-b px-2 py-1 align-top font-semibold" />; },
      td: ({ node, ...props }) => { void node; return <td {...props} className="px-2 py-1 align-top" />; },
    }}>{children}</ReactMarkdown>
  </div>;
}

function MessageBubble({ message }: { message: ConversationMessage }) {
  if (message.role === "tool") return null;
  // A turn that only asks for tools carries no text of its own. It is kept in
  // history so the next request can replay the tool calls, but drawing it would
  // put an empty bubble above the answer it leads to. Anything else that arrives
  // blank still gets a bubble, so a missing answer stays visible rather than
  // vanishing without a trace.
  if (message.role === "assistant" && message.tool_calls.length > 0 && !message.content.trim()) return null;
  return (
    <div className={`flex ${message.role === "user" ? "justify-end" : "justify-start"}`}>
      <div className={`min-w-0 max-w-[85%] break-words rounded-2xl px-4 py-3 text-sm [overflow-wrap:anywhere] ${message.role === "user" ? "bg-primary text-primary-foreground" : "bg-muted"}`}>
        {message.role === "assistant" ? <Markdown>{message.content}</Markdown> : <p className="whitespace-pre-wrap">{message.content}</p>}
      </div>
    </div>
  );
}

export function ChatThread({ messages, stream }: { messages: ConversationMessage[]; stream?: StreamState }) {
  const showStream = Boolean(stream && stream.status !== "idle" && (stream.status === "streaming" || stream.text || stream.steps.length));
  const viewport = useRef<HTMLDivElement>(null);
  // Whether the reader was at the end *before* this update landed. It has to be
  // recorded from their own scrolling: measuring once the new content is already
  // in the DOM reads the gap that content just created, which would open a saved
  // conversation at its oldest message and would let one long chunk switch
  // following off for the rest of an answer.
  const followingEnd = useRef(true);
  useEffect(() => {
    const element = viewport.current;
    if (!element || !followingEnd.current) return;
    element.scrollTop = element.scrollHeight;
  }, [messages.length, stream?.text, stream?.steps.length]);
  return (
    <Card className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <CardContent
        className="flex-1 space-y-4 overflow-y-auto pt-6"
        aria-live="polite"
        aria-label="Conversation"
        // The transcript scrolls on its own, so it has to be reachable by
        // keyboard for anyone who cannot drag a scrollbar.
        tabIndex={0}
        ref={viewport}
        onScroll={(event) => {
          const element = event.currentTarget;
          followingEnd.current = element.scrollHeight - element.scrollTop - element.clientHeight <= 80;
        }}
      >
        {!messages.length && !stream?.text ? (
          <div className="grid min-h-72 place-items-center text-center text-muted-foreground"><div><Bot className="mx-auto mb-3 size-8" /><p>Send your first request to the assistant.</p></div></div>
        ) : messages.map((message) => <MessageBubble key={message.id} message={message} />)}

        {stream && showStream ? (
          <div className="min-w-0 max-w-[85%] break-words space-y-3 rounded-2xl bg-muted px-4 py-3 text-sm [overflow-wrap:anywhere]">
            {stream.text ? <Markdown>{stream.text}</Markdown> : <span className="flex items-center gap-2 text-muted-foreground"><LoaderCircle className="size-4 animate-spin" />Preparing a response…</span>}
            {stream.steps.map((step) => (
              <Badge key={step.callId} variant="outline" className="mr-2 gap-1">
                {step.status === "running" ? <LoaderCircle className="size-3 animate-spin" /> : step.status === "succeeded" ? <CheckCircle2 className="size-3" /> : <XCircle className="size-3" />}
                <Wrench className="size-3" />{step.status === "running" ? "Using tool" : "Used tool"}: {step.name}
              </Badge>
            ))}
            {stream.steps.length ? (
              <Collapsible><CollapsibleTrigger className="text-xs underline">Technical details</CollapsibleTrigger><CollapsibleContent className="mt-2 text-xs text-muted-foreground">{stream.steps.length} tool actions in this response.</CollapsibleContent></Collapsible>
            ) : null}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
