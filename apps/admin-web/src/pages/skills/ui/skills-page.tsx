import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { Archive, BookOpen, FileText, LibraryBig, Trash2, Upload } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router";
import { toast } from "sonner";

import { skillQueries, type SkillSummary } from "@/entities/skill";
import { DeleteSkillDialog } from "@/features/skill-delete";
import { DownloadSkillButton, onlyInstructionsKept } from "@/features/skill-download";
import { importSkill } from "@/features/skill-import";
import { useAuthSession } from "@/shared/api";
import { formatDate } from "@/shared/lib";
import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, Input, Label } from "@/shared/ui";

export function SkillsPage() {
  const skills = useInfiniteQuery(skillQueries.list());
  const client = useQueryClient();
  const canEdit = useAuthSession().user?.role !== "member";
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<SkillSummary | null>(null);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const detail = useQuery({ ...skillQueries.detail(selectedId ?? ""), enabled: Boolean(selectedId) });
  const items = skills.data?.pages.flatMap((page) => page.items) ?? [];

  const upload = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    if (!uploadFile || uploadFile.size === 0) {
      toast.error("Choose a Markdown or ZIP file");
      return;
    }
    setUploading(true);
    try {
      await importSkill(uploadFile);
      await client.invalidateQueries({ queryKey: ["skills"] });
      form.reset();
      setUploadFile(null);
      toast.success("Skill added to the hub");
    } catch (error) {
      toast.error("Unable to add skill", { description: errorMessage(error, "Check the file format and try again.") });
    } finally {
      setUploading(false);
    }
  };

  const afterDelete = async (skill: SkillSummary) => {
    await client.invalidateQueries({ queryKey: ["skills"] });
    if (selectedId === skill.id) setSelectedId(null);
  };

  return <main className="space-y-6">
    <div className="flex justify-end">
      <Button asChild variant="outline"><Link to="/agents"><BookOpen />Manage assistant skills</Link></Button>
    </div>

    {canEdit ? <Card><CardHeader><CardTitle>Add a skill</CardTitle><CardDescription>Upload one Markdown file, or a ZIP containing exactly one SKILL.md. Files are validated before they become available.</CardDescription></CardHeader><CardContent><form className="flex flex-col gap-3 sm:flex-row sm:items-end" onSubmit={upload}><Label className="grid flex-1 gap-2">Skill file<Input accept=".md,.markdown,.zip,text/markdown,application/zip" disabled={uploading} name="skill-file" onChange={(event) => setUploadFile(event.currentTarget.files?.[0] ?? null)} required type="file" /></Label><Button disabled={uploading} type="submit"><Upload />{uploading ? "Adding…" : "Add to Skill Hub"}</Button></form><p className="mt-3 text-xs text-muted-foreground">Maximum upload: 2 MiB. The usable SKILL.md content must be UTF-8 and no larger than 100 KiB.</p></CardContent></Card> : <p className="text-sm text-muted-foreground">You can view available skills. Contact an administrator to add or remove them.</p>}

    {skills.isError ? <Card><CardContent className="pt-6">Unable to load skills. <Button variant="link" onClick={() => void skills.refetch()}>Try again</Button></CardContent></Card> : null}
    {skills.isLoading ? <p className="text-sm text-muted-foreground">Loading skills…</p> : null}
    {skills.isSuccess && items.length === 0 ? <Card><CardHeader><LibraryBig /><CardTitle>No skills yet</CardTitle><CardDescription>{canEdit ? "Add a Markdown file or ZIP package to build your shared instruction library." : "An administrator has not added any skills yet."}</CardDescription></CardHeader></Card> : null}

    <div className="grid gap-4 lg:grid-cols-2">{items.map((skill) => <Card key={skill.id}><CardHeader><div className="flex items-start justify-between gap-3"><div className="flex min-w-0 gap-3"><span className="grid size-10 shrink-0 place-items-center rounded-xl bg-accent">{skill.source_type === "zip" ? <Archive className="size-5" /> : <FileText className="size-5" />}</span><div className="min-w-0"><CardTitle className="truncate">{skill.name}</CardTitle><CardDescription className="mt-1 line-clamp-2">{skill.description || "No description provided."}</CardDescription></div></div><Badge variant="secondary">{skill.source_type === "zip" ? "ZIP" : "Markdown"}</Badge></div></CardHeader><CardContent className="space-y-4"><div className="grid gap-1 text-xs text-muted-foreground"><span className="truncate">Source: {skill.source_filename}</span><span>Added {formatDate(skill.created_at)}</span></div><div className="flex flex-wrap gap-2"><Button aria-label={`View instructions for ${skill.name}`} variant="outline" onClick={() => setSelectedId(skill.id)}>View instructions</Button><DownloadSkillButton skill={skill} />{canEdit ? <Button aria-label={`Delete ${skill.name}`} variant="ghost" className="text-destructive hover:text-destructive" onClick={() => setPendingDelete(skill)}><Trash2 />Delete</Button> : null}</div></CardContent></Card>)}</div>
    {skills.hasNextPage ? <div className="flex justify-center"><Button disabled={skills.isFetchingNextPage} variant="outline" onClick={() => void skills.fetchNextPage()}>{skills.isFetchingNextPage ? "Loading more…" : "Load more skills"}</Button></div> : null}

    <DeleteSkillDialog skill={pendingDelete} onOpenChange={(open) => { if (!open) setPendingDelete(null); }} onDeleted={() => { const skill = pendingDelete; if (skill) void afterDelete(skill); }} />

    <Dialog open={Boolean(selectedId)} onOpenChange={(open) => { if (!open) setSelectedId(null); }}><DialogContent className="max-h-[86vh] max-w-3xl"><DialogHeader><DialogTitle>{detail.data?.name ?? "Skill instructions"}</DialogTitle><DialogDescription>{detail.data ? `${detail.data.source_filename} · checksum ${detail.data.checksum.slice(0, 12)}…` : "Review the canonical Markdown that will be provided to assistants."}</DialogDescription></DialogHeader>{detail.isLoading ? <p>Loading instructions…</p> : detail.isError ? <p className="text-sm text-destructive">Unable to load these instructions. <Button variant="link" onClick={() => void detail.refetch()}>Try again</Button></p> : <>{detail.data && onlyInstructionsKept(detail.data) ? <p className="text-xs text-muted-foreground">This skill was added before full packages were kept, so Download saves only these SKILL.md instructions.{canEdit ? " To keep the whole package, delete this skill, add the ZIP again, then turn the skill back on for each assistant that used it." : ""}</p> : null}<pre className="max-h-[56vh] overflow-auto whitespace-pre-wrap rounded-xl border bg-muted/40 p-4 text-sm leading-6">{detail.data?.content}</pre>{detail.data ? <div className="flex justify-end"><DownloadSkillButton skill={detail.data} /></div> : null}</>}</DialogContent></Dialog>
  </main>;
}

function errorMessage(error: unknown, fallback: string) {
  if (typeof error === "object" && error !== null && "detail" in error && typeof error.detail === "string") return error.detail;
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}
