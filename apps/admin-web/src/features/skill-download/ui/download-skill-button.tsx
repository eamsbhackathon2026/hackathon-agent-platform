import { Download } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import type { SkillSummary } from "@/entities/skill";
import { downloadFile } from "@/shared/lib";
import { Button } from "@/shared/ui";
import { fetchSkillFile } from "../api/download-skill";
import { onlyInstructionsKept, skillFileName } from "../lib/skill-file-name";

/** Downloads the file the skill was added from, which "Add a skill" accepts again. */
export function DownloadSkillButton({ skill, variant = "outline" }: { skill: SkillSummary; variant?: "outline" | "ghost" }) {
  const [pending, setPending] = useState(false);
  const download = async () => {
    setPending(true);
    try {
      const fileName = skillFileName(skill);
      const type = fileName.toLowerCase().endsWith(".zip") ? "application/zip" : "text/markdown;charset=utf-8";
      downloadFile(await fetchSkillFile(skill.id), fileName, type);
    } catch {
      toast.error("Unable to download skill", { description: "Check your connection and try again." });
    } finally {
      setPending(false);
    }
  };
  return <Button aria-label={`Download ${skill.name}`} disabled={pending} title={onlyInstructionsKept(skill) ? "Downloads SKILL.md only. The original ZIP was not kept for skills added before package downloads existed." : undefined} variant={variant} onClick={() => void download()}>
    <Download />{pending ? "Downloading…" : "Download"}
  </Button>;
}
