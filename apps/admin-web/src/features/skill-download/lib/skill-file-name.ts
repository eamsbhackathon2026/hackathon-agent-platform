import type { SkillSummary } from "@/entities/skill";

/**
 * Mirrors the server's download name so the page does not depend on reading response
 * headers. A kept upload keeps its name; otherwise the stored SKILL.md is named after the
 * upload's stem, which is also the name a re-import falls back to.
 */
export function skillFileName(skill: Pick<SkillSummary, "name" | "source_type" | "source_filename" | "source_file_available">) {
  const source = skill.source_filename.trim();
  if (skill.source_file_available || skill.source_type === "markdown" && /\.(md|markdown)$/i.test(source)) return source;
  const stem = source.replace(/\.[^.]*$/, "") || skill.name.trim();
  return `${stem}.md`;
}

/** A ZIP skill imported before uploads were kept can only return its SKILL.md. */
export function onlyInstructionsKept(skill: Pick<SkillSummary, "source_type" | "source_file_available">) {
  return skill.source_type === "zip" && !skill.source_file_available;
}
