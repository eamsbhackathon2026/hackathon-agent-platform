import { useQuery, useQueryClient } from "@tanstack/react-query";
import { PlugZap, Plus } from "lucide-react";
import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { toast } from "sonner";
import { GREENNODE_MODELS, ProviderModelPicker, ProviderReadinessBadge, providerKindLabel, providerQueries, type Provider, type ProviderKind, type ProviderModel } from "@/entities/provider";
import { deleteProvider, relatedAgents } from "@/features/provider-delete";
import { testProvider, providerTestAction } from "@/features/provider-test-connection";
import { createProvider, updateProvider } from "@/features/provider-upsert";
import { apiClient, useAuthSession } from "@/shared/api";
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input, Label } from "@/shared/ui";

export function ConnectionsPage() {
  const providers = useQuery(providerQueries.list());
  const client = useQueryClient();
  const navigate = useNavigate();
  const [search] = useSearchParams();
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Provider | null>(null);
  const [models, setModels] = useState<ProviderModel[]>([]);
  const [kind, setKind] = useState<ProviderKind>("gemini");
  const [defaultModel, setDefaultModel] = useState("");
  const [replacingApiKey, setReplacingApiKey] = useState(false);
  const [conflict, setConflict] = useState<{ id: string; name: string }[]>([]);
  const canEdit = useAuthSession().user?.role !== "member";
  const refresh = () => client.invalidateQueries({ queryKey: ["providers"] });

  const selectKind = (next: ProviderKind) => {
    setKind(next);
    const suggestions = next === "greennode" ? GREENNODE_MODELS : [];
    setModels(suggestions);
    setDefaultModel(suggestions[0]?.id ?? "");
  };
  const openCreate = () => {
    setEditing(null);
    setModels([]);
    setKind("gemini");
    setDefaultModel("");
    setReplacingApiKey(false);
    setShowForm(true);
  };
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const values = new FormData(event.currentTarget);
    const apiKeyValue = values.get("api_key");
    const apiKey = typeof apiKeyValue === "string" ? apiKeyValue : "";
    const body = {
      name: String(values.get("name")),
      kind,
      base_url: kind === "openai_compatible" ? String(values.get("base_url")) : null,
      default_model: defaultModel || null,
    };
    try {
      if (editing) {
        await updateProvider(editing.id, { ...body, ...(apiKey ? { api_key: apiKey } : {}) });
        await refresh();
        setShowForm(false);
        setEditing(null);
        setReplacingApiKey(false);
        toast.success("Model connection updated");
        return;
      }
      const created = await createProvider({ ...body, api_key: apiKey });
      await refresh();
      setShowForm(false);
      toast.success("Model connection added", { action: { label: "Use for a new assistant", onClick: () => navigate(`/agents/new?provider=${created.id}`) } });
      const returnTo = search.get("returnTo");
      if (returnTo) navigate(`${returnTo}${returnTo.includes("?") ? "&" : "?"}provider=${created.id}`);
    } catch {
      toast.error("Unable to save connection", { description: "Check the credentials and address, then try again." });
    }
  };
  const beginEdit = async (provider: Provider) => {
    setEditing(provider);
    setKind(provider.kind);
    setModels([]);
    setDefaultModel(provider.default_model ?? "");
    setReplacingApiKey(false);
    setShowForm(true);
    if (provider.kind === "greennode") {
      setModels(GREENNODE_MODELS);
      return;
    }
    try {
      const { data, error } = await apiClient.GET("/v1/providers/{providerId}/models", { params: { path: { providerId: provider.id } } });
      if (!data) throw error;
      setModels(data.items);
    } catch {
      setModels([]);
      toast.error("Unable to load models", { description: "You can still enter a model name manually.", action: { label: "Try again", onClick: () => void beginEdit(provider) } });
    }
  };
  const runTest = async (provider: Provider) => {
    try {
      const result = await testProvider(provider.id);
      if (result.ok) toast.success("Connection is working");
      else {
        const action = providerTestAction(result.code);
        toast.error(action.message, { action: { label: action.action, onClick: () => void beginEdit(provider) } });
      }
      await refresh();
    } catch {
      toast.error("Unable to test connection", { action: { label: "Try again", onClick: () => void runTest(provider) } });
    }
  };
  const remove = async (provider: Provider) => {
    if (!window.confirm(`Delete connection “${provider.name}”?`)) return;
    try {
      await deleteProvider(provider.id);
      await refresh();
      toast.success("Connection deleted");
    } catch (error) {
      const related = relatedAgents(error);
      if (related.length) {
        setConflict(related);
        toast.error("This connection is used by an assistant", { description: "Move the assistants to another connection before deleting it." });
      } else toast.error("Unable to delete connection", { action: { label: "Try again", onClick: () => void remove(provider) } });
    }
  };

  return <main className="space-y-6">
    {canEdit ? <div className="flex justify-end"><Button onClick={() => showForm ? setShowForm(false) : openCreate()}><Plus />Add connection</Button></div> : null}
    {!canEdit ? <p className="text-sm text-muted-foreground">Only administrators can edit connections. Contact an administrator for help.</p> : null}
    {providers.isError ? <Card><CardContent className="pt-6">Unable to load connections. <Button variant="link" onClick={() => void providers.refetch()}>Try again</Button></CardContent></Card> : null}
    {showForm ? <Card><CardHeader><CardTitle>{editing ? "Edit connection" : "New connection"}</CardTitle><CardDescription>Choose a service. Credentials are stored securely and are never displayed again.</CardDescription></CardHeader><CardContent><form key={editing?.id ?? "new"} autoComplete="off" className="grid gap-4 md:grid-cols-2" onSubmit={submit}>
      <Field label="Service type"><select name="kind" value={kind} onChange={(event) => selectKind(event.target.value as ProviderKind)} className="h-10 rounded-md border bg-background px-3"><option value="gemini">Google Gemini</option><option value="greennode">GreenNode</option><option value="openai_compatible">OpenAI-compatible AI server — for example, your company's internal AI</option></select></Field>
      <Field label="Connection name"><Input name="name" defaultValue={editing?.name} required /></Field>
      {kind === "openai_compatible" ? <Field label="Server address"><Input name="base_url" type="url" autoComplete="url" defaultValue={editing?.base_url ?? ""} required /></Field> : null}
      <ProviderModelPicker display={editing ? "combobox" : "cards"} label="Default model" name="default_model" models={models} value={defaultModel} onChange={setDefaultModel} />
      <ApiKeyField editing={Boolean(editing)} replacing={replacingApiKey} onCancel={() => setReplacingApiKey(false)} onReplace={() => setReplacingApiKey(true)} />
      <div className="self-end"><Button type="submit">Save connection</Button></div>
    </form></CardContent></Card> : null}
    {conflict.length ? <Card><CardHeader><CardTitle>Assistants need another connection</CardTitle></CardHeader><CardContent className="space-y-2">{conflict.map((item) => <Button key={item.id} asChild variant="link"><Link to={`/agents/${item.id}`}>{item.name}</Link></Button>)}</CardContent></Card> : null}
    {providers.isSuccess && !providers.data.length ? <Card><CardHeader><PlugZap /><CardTitle>No connections yet</CardTitle><CardDescription>Add Google Gemini, GreenNode, or your company's AI server to get started.</CardDescription></CardHeader></Card> : null}
    <div className="grid gap-4 lg:grid-cols-2">{providers.data?.map((item) => <Card key={item.id}><CardHeader><div className="flex justify-between"><CardTitle>{item.name}</CardTitle><ProviderReadinessBadge status={item.status} /></div><CardDescription>{providerKindLabel(item.kind)}</CardDescription></CardHeader>{canEdit ? <CardContent className="flex gap-2"><Button variant="outline" onClick={() => void runTest(item)}>Test connection</Button><Button variant="ghost" onClick={() => void beginEdit(item)}>Edit</Button><Button variant="destructive" onClick={() => void remove(item)}>Delete</Button></CardContent> : null}</Card>)}</div>
  </main>;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <Label className="grid gap-2">{label}{children}</Label>;
}

function ApiKeyField({ editing, replacing, onReplace, onCancel }: { editing: boolean; replacing: boolean; onReplace: () => void; onCancel: () => void }) {
  if (editing && !replacing) return <div className="grid gap-2">
    <Label>API key</Label>
    <Button aria-label="Update API key" className="h-11 justify-between px-3.5 font-normal" type="button" variant="outline" onClick={onReplace}>
      <span aria-hidden="true" className="font-mono tracking-[0.18em] text-muted-foreground">••••••••••••</span>
      <span className="text-xs font-semibold text-primary">Update</span>
    </Button>
    <p className="text-xs text-muted-foreground">Your saved key stays hidden. Click to replace it.</p>
  </div>;

  return <div className="grid gap-2">
    <Label htmlFor="connection-api-key">API key</Label>
    <div className="flex gap-2">
      <Input autoComplete="new-password" autoFocus={editing} id="connection-api-key" name="api_key" placeholder={editing ? "Enter a new API key" : undefined} required={!editing} type="password" />
      {editing ? <Button type="button" variant="ghost" onClick={onCancel}>Cancel</Button> : null}
    </div>
    {editing ? <p className="text-xs text-muted-foreground">The current key is kept until you save a new one.</p> : null}
  </div>;
}
