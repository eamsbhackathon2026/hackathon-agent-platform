import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { LibraryBig } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router";
import { toast } from "sonner";

import { skillQueries } from "@/entities/skill";
import { replaceAgentSkills } from "@/features/agent-skills-bind";
import { apiClient, queryKeys } from "@/shared/api";
import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Checkbox, Label } from "@/shared/ui";

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
  const [draft, setDraft] = useState<{ agentId: string; ids: string[] } | null>(null);
  const [savingAgentId, setSavingAgentId] = useState<string | null>(null);
  const items = skills.data?.pages.flatMap((page) => page.items) ?? [];
  const selectedIds = draft?.agentId === agentId ? draft.ids : bindings.data?.skill_ids ?? [];
  const changed = draft?.agentId === agentId;
  const saving = savingAgentId === agentId;
  const toggle = (id: string) => setDraft({ agentId, ids: selectedIds.includes(id) ? selectedIds.filter((value) => value !== id) : [...selectedIds, id] });
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
    {ready && items.length > 0 ? <div className="grid gap-2 md:grid-cols-2">{items.map((skill) => <Label key={skill.id} className="flex min-h-16 items-start gap-3 rounded-xl border p-3"><Checkbox aria-label={skill.name} checked={selectedIds.includes(skill.id)} disabled={readOnly || saving || (!selectedIds.includes(skill.id) && selectedIds.length >= 20)} onCheckedChange={() => toggle(skill.id)} /><span className="min-w-0"><span className="block font-medium">{skill.name}</span><span className="mt-0.5 line-clamp-2 text-xs font-normal text-muted-foreground">{skill.description || skill.source_filename}</span></span></Label>)}</div> : null}
    {skills.hasNextPage ? <Button disabled={skills.isFetchingNextPage} variant="outline" onClick={() => void skills.fetchNextPage()}>{skills.isFetchingNextPage ? "Loading more…" : "Load more skills"}</Button> : null}
    {ready && items.length > 0 ? <div className="flex flex-wrap gap-2">{!readOnly ? <Button disabled={!changed || saving} onClick={() => void save()} type="button">{saving ? "Saving…" : "Save skill list"}</Button> : null}<Button asChild variant="outline"><Link to="/skills">View Skill Hub</Link></Button></div> : null}
  </CardContent></Card>;
}

function errorMessage(error: unknown, fallback: string) {
  if (typeof error === "object" && error !== null && "detail" in error && typeof error.detail === "string") return error.detail;
  return fallback;
}
