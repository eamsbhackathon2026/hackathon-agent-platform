import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { toast } from "sonner";
import { apiConnectionQueries, type ApiConnection } from "@/entities/api-connection";
import { toolQueries, type HttpTool, type ToolParam } from "@/entities/tool";
import { createHttpTool, updateHttpTool } from "@/features/tool-http-upsert";
import { useAuthSession } from "@/shared/api";
import { headerRows, headersRecord, secretHeadersPatch, usePageHeader, validateDistinctHeaderNames, type HeaderRow } from "@/shared/lib";
import { Button, Card, CardContent, Skeleton } from "@/shared/ui";
import { ToolConfigForm } from "@/widgets/tool-config-form";

/**
 * One page serves both /tools/new and /tools/:toolId/edit. A tool carries enough
 * configuration (inputs, nested shapes, headers, connection details) that a dialog
 * became cramped; a page gives the form room and a stable URL to return to.
 */
export function ToolEditPage() {
  const { toolId } = useParams(); const navigate = useNavigate(); const [search] = useSearchParams(); const session = useAuthSession();
  const returnTo = search.get("returnTo");
  const connections = useQuery(apiConnectionQueries.list());
  const tool = useQuery({ ...toolQueries.detail(toolId ?? ""), enabled: Boolean(toolId) });
  const canEdit = session.user?.role !== "member";
  // Keyed off the route, not the loaded tool, so an edit route never flashes the create title.
  // A member only sees the "ask an administrator" card, so the header keeps the section title.
  usePageHeader(canEdit ? { title: toolId ? "Edit tool" : "Create HTTP tool", description: "Describe each input clearly so the assistant can use this tool correctly." } : null);
  if (!canEdit) return <Card><CardContent className="space-y-3 pt-6"><p>Only administrators can edit tools. Contact an administrator for help.</p><Button asChild variant="outline"><Link to="/tools">Back to tools</Link></Button></CardContent></Card>;
  if (toolId && tool.isPending) return <main className="space-y-4"><Skeleton className="h-8 w-56" /><Skeleton className="h-96 w-full" /></main>;
  if (toolId && (tool.isError || !tool.data)) return <Card><CardContent className="space-y-3 pt-6"><p>Unable to load this tool. It may have been deleted.</p><div className="flex gap-2"><Button variant="outline" onClick={() => void tool.refetch()}>Try again</Button><Button asChild variant="ghost"><Link to="/tools">Back to tools</Link></Button></div></CardContent></Card>;
  const editing = toolId ? tool.data ?? null : null;
  return <main className="space-y-6">
    <Button asChild variant="link" className="h-auto p-0 text-muted-foreground"><Link to={returnTo ?? "/tools"}>← Back</Link></Button>
    <Card><CardContent className="pt-6"><ToolEditor key={editing?.id ?? "new"} editing={editing} connections={connections.data ?? []} onDone={(saved, wasEditing) => { if (wasEditing) { navigate(returnTo ?? "/tools"); return; } const params = new URLSearchParams({ created: saved.id }); if (returnTo) params.set("returnTo", returnTo); navigate(`/tools?${params}`); }} onCancel={() => navigate(returnTo ?? "/tools")} /></CardContent></Card>
  </main>;
}

type EditorProps = { editing: HttpTool | null; connections: ApiConnection[]; onDone: (saved: HttpTool, wasEditing: boolean) => void; onCancel: () => void };

// Holds the form's editable collections; remounted via `key` whenever the edited tool changes.
function ToolEditor({ editing, connections, onDone, onCancel }: EditorProps) {
  const client = useQueryClient();
  const [params, setParams] = useState<ToolParam[]>(editing?.params ?? []);
  const [publicHeaders, setPublicHeaders] = useState<HeaderRow[]>(editing ? headerRows(editing.public_headers) : []);
  const [secretHeaders, setSecretHeaders] = useState<HeaderRow[]>(editing ? editing.secret_header_names.map((name) => ({ id: crypto.randomUUID(), name, value: "" })) : []);
  const [replaceSecrets, setReplaceSecrets] = useState(false);
  const save = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault(); const values = new FormData(event.currentTarget);
    try {
      const publicHeaderValues = headersRecord(publicHeaders);
      const secretPatch = secretHeadersPatch(Boolean(editing), replaceSecrets, secretHeaders);
      validateDistinctHeaderNames(publicHeaderValues, secretPatch.secret_headers);
      const connectionId = String(values.get("connection_id"));
      const body = { slug: String(values.get("slug")), display_name: String(values.get("display_name")), description: String(values.get("description")), method: String(values.get("method")) as HttpTool["method"], url_template: String(values.get("url_template")), connection_id: connectionId || null, timeout_seconds: editing?.timeout_seconds ?? 15, params, public_headers: publicHeaderValues, ...secretPatch };
      const saved = editing ? await updateHttpTool(editing.id, body) : await createHttpTool(body);
      await client.invalidateQueries({ queryKey: ["tools"] });
      toast.success(editing ? "Tool updated" : "Tool created");
      onDone(saved, Boolean(editing));
    } catch (error) {
      toast.error("Unable to save tool", { description: error instanceof Error ? error.message : "Review the information and try again." });
    }
  };
  return <ToolConfigForm editing={editing} connections={connections} publicHeaders={publicHeaders} secretHeaders={secretHeaders} replaceSecrets={replaceSecrets} onParams={setParams} onPublicHeaders={setPublicHeaders} onSecretHeaders={setSecretHeaders} onReplaceSecrets={setReplaceSecrets} onSubmit={save} onCancel={onCancel} />;
}
