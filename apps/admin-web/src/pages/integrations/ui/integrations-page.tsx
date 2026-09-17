import { useQuery, useQueryClient } from "@tanstack/react-query";
import { KeyRound, Plus } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { agentQueries } from "@/entities/agent";
import { apiKeyQueries, type ApiKeyCreated } from "@/entities/api-key";
import { createApiKey } from "@/features/api-key-create";
import { revokeApiKey } from "@/features/api-key-revoke";
import { formatDate } from "@/shared/lib";
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Checkbox, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, Input, Label } from "@/shared/ui";
import { ApiAccessGuide } from "@/widgets/api-access-guide";
import { ApiKeyCreatedDialog } from "@/widgets/api-key-created-dialog";

export function IntegrationsPage() {
  const keys = useQuery(apiKeyQueries.list()); const agents = useQuery(agentQueries.list()); const client = useQueryClient(); const [showForm, setShowForm] = useState(false); const [created, setCreated] = useState<ApiKeyCreated | null>(null); const [agentId, setAgentId] = useState("");
  const submit = async (event: React.FormEvent<HTMLFormElement>) => { event.preventDefault(); if (!agentId) return toast.error("Choose an assistant before creating a key"); const values = new FormData(event.currentTarget); const scopes = [values.get("write") ? "runs:write" : null, values.get("read") ? "runs:read" : null].filter(Boolean) as ("runs:write" | "runs:read")[]; if (!scopes.length) return toast.error("Choose at least one permission"); try { const result = await createApiKey({ name: String(values.get("name")), scopes }); setCreated(result); setShowForm(false); await client.invalidateQueries({ queryKey: ["api-keys"] }); } catch { toast.error("Unable to create access key", { description: "Try again or check your account permissions.", action: { label: "Try again", onClick: () => setShowForm(true) } }); } };
  const revoke = async (id: string) => { if (!window.confirm("Revoke this key? Applications using it will stop working immediately.")) return; try { await revokeApiKey(id); await client.invalidateQueries({ queryKey: ["api-keys"] }); toast.success("Access key revoked"); } catch { toast.error("Unable to revoke access key", { action: { label: "Try again", onClick: () => void revoke(id) } }); } };
  return <main className="space-y-6"><div className="flex justify-end"><Button onClick={() => setShowForm(true)}><Plus />Create key</Button></div>
    <ApiAccessGuide onCreateKey={() => setShowForm(true)} />
    {keys.isError || agents.isError ? <Card><CardContent className="pt-6">Unable to load API access data. <Button variant="link" onClick={() => { void keys.refetch(); void agents.refetch(); }}>Try again</Button></CardContent></Card> : null}
    <Dialog open={showForm} onOpenChange={setShowForm}><DialogContent><DialogHeader><DialogTitle>New access key</DialogTitle><DialogDescription>Grant only the permissions your application needs. The secret key is shown once.</DialogDescription></DialogHeader><form className="space-y-6" onSubmit={submit}><Label className="grid gap-2">Key name<Input name="name" required placeholder="Sales website" /></Label><div className="space-y-2"><Label className="flex items-center gap-2"><Checkbox name="write" />Send requests to assistants</Label><Label className="flex items-center gap-2"><Checkbox name="read" />View status and results</Label></div><Label className="grid gap-2">Assistant used in sample requests<select required className="h-10 rounded-md border bg-background px-3" value={agentId} onChange={(event) => setAgentId(event.target.value)}><option value="">Choose assistant</option>{agents.data?.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}</select></Label><DialogFooter><Button type="button" variant="outline" onClick={() => setShowForm(false)}>Cancel</Button><Button type="submit">Create and view key</Button></DialogFooter></form></DialogContent></Dialog>
    {keys.isSuccess && !keys.data.length ? <Card><CardHeader><KeyRound /><CardTitle>No access keys yet</CardTitle><CardDescription>Create the first key when you want to call an assistant from another application.</CardDescription></CardHeader></Card> : keys.isSuccess ? <div className="space-y-3">{keys.data.map((key) => <Card key={key.id}><CardContent className="flex items-center justify-between gap-4 pt-6"><div><p className="font-medium">{key.name}</p><p className="text-sm text-muted-foreground">Starts with {key.prefix} · Last used: {key.last_used_at ? formatDate(key.last_used_at) : "Never"}</p></div><Button variant="destructive" disabled={Boolean(key.revoked_at)} onClick={() => revoke(key.id)}>{key.revoked_at ? "Revoked" : "Revoke"}</Button></CardContent></Card>)}</div> : null}
    <ApiKeyCreatedDialog created={created} agentId={agentId} onClose={() => setCreated(null)} />
  </main>;
}
