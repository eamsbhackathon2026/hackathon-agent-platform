import { Link } from "react-router";

import type { Conversation } from "@/entities/conversation";
import { runStatusLabel, runStatusVariant } from "@/entities/run";
import { DeleteConversationButton } from "@/features/delete-conversation";
import { formatDate, formatDuration } from "@/shared/lib";
import { Badge, TableCell, TableRow } from "@/shared/ui";

function usageUnits(usage: NonNullable<Conversation["summary"]>["usage"]) {
  if (usage.input_tokens === null && usage.output_tokens === null) return null;
  return (usage.input_tokens ?? 0) + (usage.output_tokens ?? 0);
}

/** One conversation with enough of its history to judge it before opening it. */
export function ConversationRow({ conversation, assistantName }: { conversation: Conversation; assistantName: string }) {
  const summary = conversation.summary;
  const heading = conversation.title.trim() || summary?.first_message || "Untitled conversation";
  const units = summary ? usageUnits(summary.usage) : null;
  const turns = summary?.turn_count ?? 0;
  return <TableRow>
    <TableCell className="max-w-80">
      <Link className="line-clamp-1 min-w-48 font-medium hover:text-primary hover:underline" to={`/conversations/${conversation.id}`}>{heading}</Link>
      {summary?.last_message ? <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground"><span className="font-medium">{summary.last_message_role === "assistant" ? "Assistant" : "User"}:</span> {summary.last_message}</p> : null}
    </TableCell>
    <TableCell>{summary?.latest_run_status ? <Badge variant={runStatusVariant(summary.latest_run_status)}>{runStatusLabel[summary.latest_run_status]}</Badge> : <Badge variant="outline">No requests</Badge>}</TableCell>
    <TableCell className="whitespace-nowrap">
      <p className="font-medium tabular-nums">{turns} {turns === 1 ? "turn" : "turns"}</p>
      {summary?.failed_turn_count ? <p className="mt-0.5 text-xs font-medium text-destructive tabular-nums">{summary.failed_turn_count} failed</p> : null}
    </TableCell>
    <TableCell>{assistantName}</TableCell>
    <TableCell className="hidden whitespace-nowrap lg:table-cell">
      <p className="tabular-nums">{units === null ? "Usage not reported" : `${units.toLocaleString("en-US")} processing units`}</p>
      {summary?.processing_ms ? <p className="mt-0.5 text-xs text-muted-foreground tabular-nums">Processing {formatDuration(summary.processing_ms)}</p> : null}
    </TableCell>
    <TableCell className="hidden xl:table-cell">{conversation.source === "playground" ? "Admin portal" : "External system"}</TableCell>
    <TableCell className="whitespace-nowrap text-sm">{formatDate(conversation.updated_at)}</TableCell>
    <TableCell className="text-right"><DeleteConversationButton conversationId={conversation.id} /></TableCell>
  </TableRow>;
}
