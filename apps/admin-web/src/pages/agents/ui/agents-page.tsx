import { useQuery } from "@tanstack/react-query";
import { Bot, Plus } from "lucide-react";
import { Link } from "react-router";
import { agentQueries, AgentStatusBadge } from "@/entities/agent";
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/shared/ui";
import { useAuthSession } from "@/shared/api";

export function AgentsPage() {
  const agents = useQuery(agentQueries.list());
  const canEdit = useAuthSession().user?.role !== "member";
  return <main className="space-y-6"><header className="flex items-start justify-between"><div><h1 className="text-2xl font-semibold">AI Assistants</h1><p className="text-muted-foreground">Create an assistant for each workflow and test it when it is ready.</p></div>{canEdit ? <Button asChild><Link to="/agents/new"><Plus />Create assistant</Link></Button> : null}</header>{!canEdit ? <p className="text-sm text-muted-foreground">Only administrators can edit assistants. Contact an administrator for help.</p> : null}
    {agents.isError ? <Card><CardContent className="pt-6">Unable to load assistants. <Button variant="link" onClick={() => agents.refetch()}>Try again</Button></CardContent></Card> : null}
    {agents.isSuccess && !agents.data.length ? <Card><CardHeader><Bot /><CardTitle>No assistants yet</CardTitle><CardDescription>{canEdit ? "Start with an assistant for a common workflow." : "Contact an administrator to create the first assistant."}</CardDescription></CardHeader>{canEdit ? <CardContent><Button asChild><Link to="/agents/new">Create first assistant</Link></Button></CardContent> : null}</Card> : null}
    <div className="grid gap-4 lg:grid-cols-2">{agents.data?.map((agent) => <Card key={agent.id}><CardHeader><div className="flex justify-between gap-3"><CardTitle>{agent.name}</CardTitle><AgentStatusBadge ready={agent.ready} /></div><CardDescription>{agent.description || "No description"}</CardDescription></CardHeader><CardContent className="flex gap-2"><Button asChild variant="outline"><Link to={`/agents/${agent.id}`}>{canEdit ? "Edit" : "View settings"}</Link></Button>{agent.ready ? <Button asChild><Link to={`/playground?agent=${agent.id}`}>Open playground</Link></Button> : <span className="self-center text-sm text-muted-foreground">Open to finish setup</span>}</CardContent></Card>)}</div>
  </main>;
}
