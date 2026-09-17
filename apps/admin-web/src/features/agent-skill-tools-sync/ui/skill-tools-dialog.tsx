import { Link } from "react-router";

import { Alert, AlertDescription, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/shared/ui";

import type { ResolvedSkillTool } from "../lib/resolve-skill-tools";

export type SkillToolsDialogMode = "add" | "remove";

export type SkillToolsPrompt = {
  mode: SkillToolsDialogMode;
  skillName: string;
  items: ResolvedSkillTool[];
  unresolved: string[];
};

/**
 * Adding a skill and removing one are the same question asked in two directions, so both
 * use one dialog: here is what changes, confirm or leave it alone. Nothing is written
 * here — the caller updates the draft and the existing Save button commits it.
 */
export function SkillToolsDialog({ prompt, onConfirm, onDismiss }: { prompt: SkillToolsPrompt | null; onConfirm: () => void; onDismiss: () => void }) {
  const adding = prompt?.mode === "add";
  const count = prompt?.items.length ?? 0;
  const groups = groupByLabel(prompt?.items ?? []);
  return (
    <Dialog open={Boolean(prompt)} onOpenChange={(open) => { if (!open) onDismiss(); }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{adding ? "This skill needs more tools" : "Turn off tools nothing needs?"}</DialogTitle>
          <DialogDescription>
            {adding
              ? `“${prompt?.skillName}” uses ${count} ${count === 1 ? "tool" : "tools"} this assistant does not have yet.`
              : `No skill you have selected still needs ${count} ${count === 1 ? "tool" : "tools"}.`}
          </DialogDescription>
        </DialogHeader>
        <div className="max-h-64 space-y-4 overflow-y-auto">
          {groups.map(([groupLabel, items]) => (
            <section key={groupLabel} className="space-y-1" aria-label={groupLabel}>
              <h3 className="text-sm font-medium">{groupLabel}</h3>
              <ul className="space-y-1">
                {items.map((item) => (
                  <li key={`${item.kind}:${item.id}`} className="flex flex-wrap items-baseline gap-x-2 rounded-md border p-2 text-sm">
                    <span>{item.label}</span>
                    <code className="text-xs text-muted-foreground">${item.ref}</code>
                    {item.kind === "server" ? <span className="text-xs text-muted-foreground">Whole tool server</span> : null}
                  </li>
                ))}
              </ul>
            </section>
          ))}
          {prompt?.unresolved.length ? (
            <Alert>
              <AlertDescription>
                This skill mentions {prompt.unresolved.map((ref) => `$${ref}`).join(", ")}, but no tool by that name exists here yet.{" "}
                <Button asChild variant="link" className="h-auto p-0"><Link to="/tools">Open Tools</Link></Button>
              </AlertDescription>
            </Alert>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onDismiss}>{adding ? "Not now" : "Leave them on"}</Button>
          <Button onClick={onConfirm} disabled={count === 0}>{adding ? `Turn on ${count}` : `Turn off ${count}`}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function groupByLabel(items: ResolvedSkillTool[]): [string, ResolvedSkillTool[]][] {
  const groups = new Map<string, ResolvedSkillTool[]>();
  for (const item of items) groups.set(item.groupLabel, [...(groups.get(item.groupLabel) ?? []), item]);
  return [...groups.entries()].sort(([left], [right]) => left.localeCompare(right));
}
