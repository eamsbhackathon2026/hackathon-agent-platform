import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { agentQueries } from "@/entities/agent";
import { archiveAgent } from "@/features/agent-archive";
import { updateAgent } from "@/features/agent-upsert";
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Tabs, TabsContent, TabsList, TabsTrigger } from "@/shared/ui";
import { AgentForm, type AgentFormValues } from "@/widgets/agent-form";
import { AgentToolDraftProvider } from "../model";
import { AgentToolsPanel } from "./agent-tools-panel";
import { AgentSkillsPanel } from "./agent-skills-panel";
import { useAuthSession } from "@/shared/api";
import { usePageHeader } from "@/shared/lib";

export function AgentDetailPage() {
  const { agentId = "" } = useParams(); const navigate = useNavigate(); const queryClient = useQueryClient(); const agent = useQuery(agentQueries.detail(agentId)); const [saving, setSaving] = useState(false); const canEdit = useAuthSession().user?.role !== "member";
  usePageHeader(agent.data ? { title: agent.data.name, description: "Update how this assistant works or connect it to an external system." } : null);
  if (agent.isLoading) return <p>Loading assistant…</p>;
  if (agent.isError) { const notFound = errorStatus(agent.error) === 404; return <Card><CardContent className="pt-6">{notFound ? "This assistant was not found." : "Unable to load the assistant."} {notFound ? <Button asChild variant="link"><Link to="/agents">Back to list</Link></Button> : <Button variant="link" onClick={() => void agent.refetch()}>Try again</Button>}</CardContent></Card>; }
  if (!agent.data) return <p>Loading assistant…</p>;
  const save = async (values: AgentFormValues) => { setSaving(true); try { await updateAgent(agentId, values); await queryClient.invalidateQueries({ queryKey: ["agents"] }); toast.success("Changes saved"); } catch { toast.error("Unable to save changes", { description: "Review the information and try again." }); } finally { setSaving(false); } };
  const archive = async () => { if (!window.confirm("Archive this assistant? Previous activity will remain available.")) return; try { await archiveAgent(agentId); toast.success("Assistant archived"); navigate("/agents"); } catch { toast.error("Unable to archive assistant", { action: { label: "Try again", onClick: () => void archive() } }); } };
  return <main className="space-y-6">{canEdit ? <div className="flex justify-end"><Button variant="destructive" onClick={archive}>Archive</Button></div> : null}{!canEdit ? <p className="text-sm text-muted-foreground">Only administrators can edit these settings. Contact an administrator for help.</p> : null}
    <Tabs defaultValue="settings"><TabsList><TabsTrigger value="settings">Settings</TabsTrigger><TabsTrigger value="external">API access</TabsTrigger></TabsList><TabsContent value="settings" className="space-y-5"><AgentForm initial={agent.data} readOnly={!canEdit} saving={saving} onSubmit={save} /><AgentToolDraftProvider agentId={agentId}><AgentSkillsPanel agentId={agentId} readOnly={!canEdit} /><div className="mt-5"><AgentToolsPanel readOnly={!canEdit} /></div></AgentToolDraftProvider></TabsContent><TabsContent value="external"><Card><CardHeader><CardTitle>Connect your system</CardTitle><CardDescription>{canEdit ? "Create an access key, choose the appropriate permissions, and use a sample request." : "Contact an administrator to create an access key."}</CardDescription></CardHeader><CardContent className="flex gap-2">{canEdit ? <Button asChild><Link to="/integrations">Create access key</Link></Button> : null}<Button asChild variant="outline"><Link to={`/activity?agent=${agentId}`}>View API activity</Link></Button></CardContent></Card></TabsContent></Tabs>
  </main>;
}

function errorStatus(error: unknown) { return typeof error === "object" && error !== null && "status" in error && typeof error.status === "number" ? error.status : undefined; }
