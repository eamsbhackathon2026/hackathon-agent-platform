import type { RunStatus } from "./types";

export const runStatusLabel: Record<RunStatus, string> = { queued: "Queued", running: "Processing", succeeded: "Completed", failed: "Failed", cancelled: "Stopped" };

export function runStatusVariant(status: RunStatus) {
  if (status === "succeeded") return "success" as const;
  if (status === "failed") return "destructive" as const;
  if (status === "queued" || status === "running") return "warning" as const;
  return "outline" as const;
}

export function isRunActive(status: RunStatus | null | undefined) {
  return status === "queued" || status === "running";
}
