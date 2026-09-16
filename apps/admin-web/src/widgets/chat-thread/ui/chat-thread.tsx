import { Bot, CheckCircle2, LoaderCircle, Wrench, XCircle } from "lucide-react";
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
  return (
    <Card className="min-h-[28rem]">
      <CardContent className="space-y-4 pt-6" aria-live="polite">
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
