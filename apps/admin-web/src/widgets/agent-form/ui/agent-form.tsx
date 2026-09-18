import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { Link } from "react-router";
import type { Agent } from "@/entities/agent";
import { GREENNODE_MODELS, ProviderModelPicker, providerQueries, type ProviderModel } from "@/entities/provider";
import { apiClient } from "@/shared/api";
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Checkbox, Collapsible, CollapsibleContent, CollapsibleTrigger, Input, Label, Slider, Textarea } from "@/shared/ui";
import { agentFormSchema, type AgentFormValues } from "../model/agent-form-schema";

type Props = { initial?: Agent; preferredProviderId?: string | undefined; readOnly?: boolean; saving?: boolean; onSubmit: (values: AgentFormValues) => Promise<void> | void };
const defaults: AgentFormValues = { name: "", description: "", provider_id: "", model: "", system_prompt: "", temperature: 0.5, max_output_tokens: 2048, show_thinking: false, context_window_tokens: 32768, max_iterations: 8, timeout_seconds: 120 };

export function AgentForm({ initial, preferredProviderId, readOnly = false, saving, onSubmit }: Props) {
  const providers = useQuery(providerQueries.list());
  const form = useForm<AgentFormValues>({ resolver: zodResolver(agentFormSchema), defaultValues: initial ? { ...initial } : { ...defaults, provider_id: preferredProviderId ?? "" } });
  const selected = useWatch({ control: form.control, name: "provider_id" });
  const selectedModel = useWatch({ control: form.control, name: "model" });
  const temperature = useWatch({ control: form.control, name: "temperature" });
  const [loadedModels, setLoadedModels] = useState<{ providerId: string; items: ProviderModel[] } | null>(null);
  const selectedProvider = providers.data?.find((item) => item.id === selected);
  const models = selectedProvider?.kind === "greennode" ? GREENNODE_MODELS : loadedModels?.providerId === selected ? loadedModels.items : [];
  useEffect(() => {
    if (!selectedProvider || !selected) return;
    if (selectedProvider.default_model && !form.getValues("model")) form.setValue("model", selectedProvider.default_model);
    if (selectedProvider.kind === "greennode") return;
    let active = true;
    void apiClient.GET("/v1/providers/{providerId}/models", { params: { path: { providerId: selected } } }).then(({ data }) => { if (active) setLoadedModels({ providerId: selected, items: data?.items ?? [] }); });
    return () => { active = false; };
  }, [form, selected, selectedProvider]);
  if (providers.isLoading) return <p>Loading model connections…</p>;
  if (providers.isError) return <Card><CardHeader><CardTitle>Unable to load model connections</CardTitle><CardDescription>Check your network connection and try again.</CardDescription></CardHeader><CardContent><Button variant="outline" onClick={() => void providers.refetch()}>Try again</Button></CardContent></Card>;
  const providerItems = providers.data ?? [];
  if (!providerItems.length) return <Card><CardHeader><CardTitle>No model connections yet</CardTitle><CardDescription>{readOnly ? "This assistant has no available model connection." : "Add an AI service before creating an assistant."}</CardDescription></CardHeader>{!readOnly ? <CardContent><Button asChild><Link to="/connections?returnTo=/agents/new">Add connection</Link></Button></CardContent> : null}</Card>;
  const error = (name: keyof AgentFormValues) => form.formState.errors[name]?.message;
  return <form onSubmit={form.handleSubmit(onSubmit)}><fieldset disabled={readOnly} className="space-y-5">
    <Card><CardHeader><CardTitle>Details</CardTitle><CardDescription>Help people understand what this assistant does.</CardDescription></CardHeader><CardContent className="grid gap-4 md:grid-cols-2">
      <Field label="Assistant name" error={error("name")}><Input {...form.register("name")} /></Field><Field label="Short description" error={error("description")}><Input {...form.register("description")} /></Field>
    </CardContent></Card>
    <Card><CardHeader><CardTitle>AI model</CardTitle></CardHeader><CardContent className="grid gap-4 md:grid-cols-2">
      <Field label="Model connection" error={error("provider_id")}><select className="h-10 w-full rounded-md border bg-background px-3" {...form.register("provider_id")}><option value="">Choose connection</option>{providerItems.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></Field>
      <ProviderModelPicker display="combobox" error={error("model")} label="AI model" models={models} value={selectedModel} onChange={(value) => form.setValue("model", value, { shouldDirty: true, shouldValidate: true })} />
      <div className="md:col-span-2"><Field label="Assistant instructions" error={error("system_prompt")}><Textarea rows={7} placeholder="Describe the work, preferred tone, and anything the assistant should avoid…" {...form.register("system_prompt")} /></Field></div>
    </CardContent></Card>
    <Card><CardHeader><CardTitle>Tools</CardTitle><CardDescription>Attach tools after saving the assistant, or create a new tool now.</CardDescription></CardHeader><CardContent>{readOnly ? <p className="text-sm text-muted-foreground">Only administrators can change the tool list.</p> : <Button asChild variant="outline"><Link to="/tools/new?returnTo=/agents/new">Create new tool</Link></Button>}</CardContent></Card>
    <Collapsible><Card><CardHeader><CollapsibleTrigger asChild><Button type="button" variant="ghost">Advanced options</Button></CollapsibleTrigger></CardHeader><CollapsibleContent><CardContent className="grid gap-5 md:grid-cols-2">
      <Field label="Creativity"><div className="flex items-center gap-3 text-xs"><span>Precise</span><Slider min={0} max={2} step={0.1} value={[temperature ?? 0.5]} onValueChange={([value]) => form.setValue("temperature", value ?? 0.5)} /><span>Creative</span></div></Field>
      <Field label="Maximum response length" error={error("max_output_tokens")}><Input type="number" min={1} {...form.register("max_output_tokens", { setValueAs: (value) => value === "" ? null : Number(value) })} /></Field>
      <Field label="Conversation capacity" error={error("context_window_tokens")}><Input type="number" min={8192} max={2000000} step={1} list="context-window-presets" {...form.register("context_window_tokens", { valueAsNumber: true })} /><datalist id="context-window-presets"><option value={32768}>Compact · 32K</option><option value={65536}>Standard · 64K</option><option value={128000}>Large · 128K</option><option value={256000}>Extended · 256K</option><option value={1000000}>Maximum · 1M</option></datalist><span className="text-xs font-normal text-muted-foreground">Choose a suggested size or enter the model's exact token capacity. Older messages are summarized automatically near this limit.</span></Field>
      <div className="md:col-span-2"><Label className="flex items-start gap-3 rounded-md border p-3"><Checkbox className="mt-0.5" checked={form.watch("show_thinking")} onCheckedChange={(checked) => form.setValue("show_thinking", checked === true, { shouldDirty: true })} /><span className="grid gap-1"><span>Narrate the assistant's thinking</span><span className="text-xs font-normal text-muted-foreground">While the assistant works, callers receive a short summary of what it is thinking, so a product can show progress instead of a blank wait. It never changes the answer and is never stored with the conversation. Costs extra output tokens, and does nothing on models that do not summarize their thinking.</span></span></Label></div>
      <Field label="Maximum processing steps" error={error("max_iterations")}><Input type="number" min={1} max={25} {...form.register("max_iterations", { valueAsNumber: true })} /></Field>
      <Field label="Maximum wait time (seconds)" error={error("timeout_seconds")}><Input type="number" min={10} max={600} {...form.register("timeout_seconds", { valueAsNumber: true })} /></Field>
    </CardContent></CollapsibleContent></Card></Collapsible>
    {!readOnly ? <div className="flex justify-end"><Button disabled={saving} type="submit">{saving ? "Saving…" : "Save assistant"}</Button></div> : null}
  </fieldset></form>;
}
function Field({ label, error, children }: { label: string; error?: string | undefined; children: React.ReactNode }) { return <Label className="grid gap-2"><span>{label}</span>{children}{error ? <span className="text-sm text-destructive">{error}</span> : null}</Label>; }
