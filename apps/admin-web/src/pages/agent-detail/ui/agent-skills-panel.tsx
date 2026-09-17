import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { LibraryBig } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router";
import { toast } from "sonner";

import { skillQueries, type SkillSummary } from "@/entities/skill";
import { replaceAgentSkills } from "@/features/agent-skills-bind";
import { missingSkillTools, resolveSkillTools, isSelected, SkillToolsDialog, type ResolvedSkillTool, type SkillToolCatalog, type SkillToolsPrompt } from "@/features/agent-skill-tools-sync";
import { apiClient, queryKeys } from "@/shared/api";
import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Checkbox, Label } from "@/shared/ui";

import { useAgentToolCatalog, useAgentToolDraft } from "../model";

export function AgentSkillsPanel({ agentId, readOnly = false }: { agentId: string; readOnly?: boolean }) {
  const skills = useInfiniteQuery(skillQueries.list());
  const bindings = useQuery({
    queryKey: queryKeys.agentSkills(agentId),
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/agents/{agentId}/skills", { params: { path: { agentId } } });
      if (!data) throw error;
      return data;
    },
  });
  const client = useQueryClient();
  const { tools, servers, connectionNames } = useAgentToolCatalog();
  const toolDraft = useAgentToolDraft();
  const [draft, setDraft] = useState<{ agentId: string; ids: string[] } | null>(null);
  const [savingAgentId, setSavingAgentId] = useState<string | null>(null);
  const [prompt, setPrompt] = useState<SkillToolsPrompt | null>(null);
  const [pending, setPending] = useState<ResolvedSkillTool[]>([]);
  const items = skills.data?.pages.flatMap((page) => page.items) ?? [];
  const selectedIds = draft?.agentId === agentId ? draft.ids : bindings.data?.skill_ids ?? [];
  const changed = draft?.agentId === agentId;
  const saving = savingAgentId === agentId;
  const catalog: SkillToolCatalog = { tools: tools.data ?? [], servers: servers.data ?? [], connectionNames };
  const selection = { toolIds: toolDraft.toolIds, serverIds: toolDraft.serverIds };

  const toggle = (skill: SkillSummary) => {
    if (readOnly) return;
    const enabling = !selectedIds.includes(skill.id);
    const nextIds = enabling ? [...selectedIds, skill.id] : selectedIds.filter((value) => value !== skill.id);
    // The skill choice is applied before any dialog opens, so dismissing the dialog
    // leaves the tick where the person put it and only declines the tool change.
    setDraft({ agentId, ids: nextIds });
    const resolution = resolveSkillTools(skill.tool_refs, catalog);
    if (enabling) {
      const missing = missingSkillTools(resolution, selection);
      if (missing.length || resolution.unresolved.length) {
        setPending(missing);
        setPrompt({ mode: "add", skillName: skill.name, items: missing, unresolved: resolution.unresolved });
      }
      return;
    }
    // A tool another selected skill still needs stays on, so removing one skill never
    // disturbs the rest.
    const stillNeeded = new Set(
      items.filter((candidate) => nextIds.includes(candidate.id))
        .flatMap((candidate) => resolveSkillTools(candidate.tool_refs, catalog).resolved)
        .map(itemKey),
    );
    const orphaned = resolution.resolved.filter((item) => !stillNeeded.has(itemKey(item)) && isSelected(item, selection));
    if (orphaned.length) {
      setPending(orphaned);
      setPrompt({ mode: "remove", skillName: skill.name, items: orphaned, unresolved: [] });
    }
  };

  const confirmPrompt = () => {
    if (prompt?.mode === "add") toolDraft.addSelections(pending);
    if (prompt?.mode === "remove") toolDraft.removeSelections(pending);
    setPrompt(null);
    setPending([]);
  };
  const dismissPrompt = () => { setPrompt(null); setPending([]); };

  const save = async () => {
    if (saving) return;
    const requestAgentId = agentId;
    setSavingAgentId(requestAgentId);
    try {
      const saved = await replaceAgentSkills(requestAgentId, { skill_ids: selectedIds });
      client.setQueryData(queryKeys.agentSkills(requestAgentId), saved);
      setDraft((current) => current?.agentId === requestAgentId ? null : current);
      toast.success("Assistant skills updated");
    } catch (error) {
      toast.error("Unable to update skills", { description: errorMessage(error, "Choose fewer instructions and try again.") });
    } finally {
      setSavingAgentId((current) => current === requestAgentId ? null : current);
    }
  };

  const failed = skills.isError || bindings.isError;
  const ready = skills.isSuccess && bindings.isSuccess;
  return <Card><CardHeader><div className="flex items-start justify-between gap-3"><div><CardTitle>Skills</CardTitle><CardDescription>{readOnly ? "Reusable instructions currently enabled for this assistant." : "Choose the reusable instructions this assistant should follow at the start of each run."}</CardDescription></div><Badge variant="secondary">{selectedIds.length}/20 selected</Badge></div></CardHeader><CardContent className="space-y-4">
    {failed ? <p className="text-sm text-destructive">Unable to load skills. <Button variant="link" onClick={() => { void skills.refetch(); void bindings.refetch(); }}>Try again</Button></p> : null}
    {skills.isLoading || bindings.isLoading ? <p className="text-sm text-muted-foreground">Loading skills…</p> : null}
    {ready && items.length === 0 ? <div className="flex flex-col items-start gap-3 rounded-xl border border-dashed p-4"><LibraryBig className="size-5" /><p className="text-sm text-muted-foreground">No reusable instructions are available yet.</p><Button asChild variant="outline"><Link to="/skills">Open Skill Hub</Link></Button></div> : null}
    {ready && items.length > 0 ? <div className="grid gap-2 md:grid-cols-2">{items.map((skill) => {
      // A tool can also be turned off directly in the panel below, which no dialog sees,
      // so a selected skill states plainly when what it needs is no longer there.
      const missing = selectedIds.includes(skill.id) && toolDraft.loaded ? missingSkillTools(resolveSkillTools(skill.tool_refs, catalog), selection) : [];
      return <Label key={skill.id} className="flex min-h-16 items-start gap-3 rounded-xl border p-3"><Checkbox aria-label={skill.name} checked={selectedIds.includes(skill.id)} disabled={readOnly || saving || (!selectedIds.includes(skill.id) && selectedIds.length >= 20)} onCheckedChange={() => toggle(skill)} /><span className="min-w-0"><span className="block font-medium">{skill.name}</span><span className="mt-0.5 line-clamp-2 text-xs font-normal text-muted-foreground">{skill.description || skill.source_filename}</span>{missing.length ? <Badge variant="outline" className="mt-1.5 font-normal">Missing {missing.length} {missing.length === 1 ? "tool" : "tools"}</Badge> : null}</span></Label>;
    })}</div> : null}
    {skills.hasNextPage ? <Button disabled={skills.isFetchingNextPage} variant="outline" onClick={() => void skills.fetchNextPage()}>{skills.isFetchingNextPage ? "Loading more…" : "Load more skills"}</Button> : null}
    {ready && items.length > 0 ? <div className="flex flex-wrap gap-2">{!readOnly ? <Button disabled={!changed || saving} onClick={() => void save()} type="button">{saving ? "Saving…" : "Save skill list"}</Button> : null}<Button asChild variant="outline"><Link to="/skills">View Skill Hub</Link></Button></div> : null}
    <SkillToolsDialog prompt={prompt} onConfirm={confirmPrompt} onDismiss={dismissPrompt} />
  </CardContent></Card>;
}

function itemKey(item: ResolvedSkillTool) {
  return `${item.kind}:${item.id}`;
}

function errorMessage(error: unknown, fallback: string) {
  if (typeof error === "object" && error !== null && "detail" in error && typeof error.detail === "string") return error.detail;
  return fallback;
}
