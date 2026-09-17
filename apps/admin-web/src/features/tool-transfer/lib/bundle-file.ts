import { downloadFile } from "@/shared/lib";

import type { ToolBundle } from "../api/tool-transfer";

export const maxBundleBytes = 2 * 1024 * 1024;
export const maxBundleTools = 200;
export const maxBundleConnections = 50;

/** Reads a chosen file as a tool bundle, rejecting what the server would refuse anyway. */
export async function readBundleFile(file: File): Promise<ToolBundle> {
  if (file.size > maxBundleBytes) throw new Error("The file is larger than 2 MiB. Export fewer tools and try again.");
  let parsed: unknown;
  try {
    parsed = JSON.parse(await file.text());
  } catch {
    throw new Error("This file is not valid JSON. Choose a file created with Export tools.");
  }
  const bundle = parsed as Partial<ToolBundle> | null;
  if (typeof bundle !== "object" || bundle === null || bundle.format !== "agent-platform.tools") {
    throw new Error("This file was not created with Export tools. Choose an exported tools file.");
  }
  if (bundle.version !== 1) throw new Error("This tools file comes from a different version of the platform. Export it again from a matching version.");
  if (!Array.isArray(bundle.tools) || !Array.isArray(bundle.connections)) throw new Error("This tools file is incomplete. Export the tools again and use the new file.");
  if (bundle.tools.length > maxBundleTools || bundle.connections.length > maxBundleConnections) {
    throw new Error(`A tools file can hold at most ${maxBundleTools} tools and ${maxBundleConnections} API connections. Split the file and import each part.`);
  }
  return bundle as ToolBundle;
}

export function bundleFileName(date = new Date()) {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `tools-${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}.json`;
}

export function downloadBundle(bundle: ToolBundle, date = new Date()) {
  // Compact JSON keeps an exported file within the import size limit.
  downloadFile(JSON.stringify(bundle), bundleFileName(date), "application/json");
}

/** Prefers the server's explanation, then any per-field reasons, then the fallback. */
export function transferErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) return error.message;
  if (typeof error !== "object" || error === null) return fallback;
  const problem = error as { detail?: unknown; title?: unknown; fields?: { message?: unknown }[] };
  const reasons = (problem.fields ?? []).map((field) => field.message).filter((message): message is string => typeof message === "string");
  const lead = typeof problem.detail === "string" && problem.detail ? problem.detail : typeof problem.title === "string" ? problem.title : "";
  return [lead, ...reasons.slice(0, 3)].filter(Boolean).join(" ") || fallback;
}
