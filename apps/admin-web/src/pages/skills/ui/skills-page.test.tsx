import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { toast } from "sonner";
import { afterEach, describe, expect, it, vi } from "vitest";

import { clearAuthSession, setAuthSession } from "@/shared/api";
import { importSkill } from "@/features/skill-import";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";

import { SkillsPage } from "./skills-page";

vi.mock("@/features/skill-import", () => ({ importSkill: vi.fn() }));

const user = { id: "10000000-0000-4000-8000-000000000001", email: "user@example.test", name: "User", role: "admin" as const, status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };
const skill = { id: "20000000-0000-4000-8000-000000000002", name: "Clear writing", description: "Use plain language.", source_type: "markdown" as const, source_filename: "writing.md", checksum: "0".repeat(64), tool_refs: [], source_file_available: true, created_by: user.id, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };
const secondSkill = { ...skill, id: "20000000-0000-4000-8000-000000000003", name: "Review checklist", source_filename: "review.md" };

function useHandlers() {
  server.use(
    http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [skill], next_cursor: null })),
    http.get("*/v1/skills/:skillId", () => jsonResponse({ ...skill, content: "# Clear writing\n\nUse plain language." })),
  );
}

function captureDownloads() {
  const saved = { blobs: [] as Blob[], names: [] as string[] };
  Object.assign(URL, { createObjectURL: vi.fn((blob: Blob) => { saved.blobs.push(blob); return "blob:skill"; }), revokeObjectURL: vi.fn() });
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) { saved.names.push(this.download); });
  return saved;
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter><SkillsPage /></MemoryRouter></QueryClientProvider>);
}

afterEach(() => { clearAuthSession(); vi.restoreAllMocks(); vi.mocked(importSkill).mockReset(); });

describe("SkillsPage", () => {
  it("shows canonical instructions and uploads a Markdown skill", async () => {
    setAuthSession("token", user); useHandlers();
    vi.mocked(importSkill).mockResolvedValue({ ...skill, content: "# New skill" });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "View instructions for Clear writing" }));
    expect(await screen.findByText("# Clear writing", { exact: false })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    const input = screen.getByLabelText("Skill file");
    fireEvent.change(input, { target: { files: [new File(["# New skill"], "new-skill.md", { type: "text/markdown" })] } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(importSkill).toHaveBeenCalledWith(expect.objectContaining({ name: "new-skill.md" })));
  });

  it("keeps library mutation controls hidden for members", async () => {
    setAuthSession("token", { ...user, role: "member" }); useHandlers(); renderPage();
    expect(await screen.findByText("Clear writing")).toBeInTheDocument();
    expect(screen.queryByLabelText("Skill file")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Delete / })).not.toBeInTheDocument();
    expect(screen.getByText(/Contact an administrator/)).toBeInTheDocument();
  });

  it("loads subsequent skill pages without dropping earlier results", async () => {
    setAuthSession("token", user);
    server.use(
      http.get(apiUrl("/v1/skills"), ({ request }) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        return cursor === "next-page"
          ? jsonResponse({ items: [secondSkill], next_cursor: null })
          : jsonResponse({ items: [skill], next_cursor: "next-page" });
      }),
    );
    renderPage();

    expect(await screen.findByText("Clear writing")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Load more skills" }));

    expect(await screen.findByText("Review checklist")).toBeInTheDocument();
    expect(screen.getByText("Clear writing")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Load more skills" })).not.toBeInTheDocument();
  });

  it("deletes the selected skill after confirmation", async () => {
    setAuthSession("token", user);
    useHandlers();
    let deletedId = "";
    server.use(http.delete("*/v1/skills/:skillId", ({ params }) => {
      deletedId = String(params.skillId);
      return new Response(null, { status: 204 });
    }));
    renderPage();

    fireEvent.click(await screen.findByRole("button", { name: "Delete Clear writing" }));
    fireEvent.click(await screen.findByRole("button", { name: "Delete skill" }));

    await waitFor(() => expect(deletedId).toBe(skill.id));
  });

  it("asks in the page before deleting, and deletes nothing when the answer is no", async () => {
    setAuthSession("token", user);
    useHandlers();
    let called = false;
    server.use(http.delete("*/v1/skills/:skillId", () => {
      called = true;
      return new Response(null, { status: 204 });
    }));
    renderPage();

    fireEvent.click(await screen.findByRole("button", { name: "Delete Clear writing" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText(/Clear writing/)).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "Keep it" }));

    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(called).toBe(false);
  });

  it("lets a member download the file a skill was added from", async () => {
    setAuthSession("token", { ...user, role: "member" });
    const zipSkill = { ...skill, name: "Finance pack", source_type: "zip" as const, source_filename: "finance-pack.zip" };
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [zipSkill], next_cursor: null })),
      http.get("*/v1/skills/:skillId/download", () => new HttpResponse(new Uint8Array([80, 75, 3, 4]), { headers: { "Content-Type": "application/octet-stream" } })),
    );
    const saved = captureDownloads();
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Download Finance pack" }));
    await waitFor(() => expect(saved.names).toEqual(["finance-pack.zip"]));
    expect(new Uint8Array(await saved.blobs[0]!.arrayBuffer())).toEqual(new Uint8Array([80, 75, 3, 4]));
    expect(saved.blobs[0]?.type).toBe("application/zip");
    expect(screen.getByRole("button", { name: "Download Finance pack" })).not.toHaveAttribute("title");
  });

  it("explains that an older ZIP skill downloads only its instructions", async () => {
    setAuthSession("token", user);
    const legacy = { ...skill, name: "Old pack", source_type: "zip" as const, source_filename: "old-pack.zip", source_file_available: false };
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [legacy], next_cursor: null })),
      http.get("*/v1/skills/:skillId/download", () => new HttpResponse("# Old pack", { headers: { "Content-Type": "application/octet-stream" } })),
      http.get("*/v1/skills/:skillId", () => jsonResponse({ ...legacy, content: "# Old pack" })),
    );
    const saved = captureDownloads();
    renderPage();
    expect(await screen.findByRole("button", { name: "Download Old pack" })).toHaveAttribute("title", expect.stringContaining("SKILL.md only"));
    fireEvent.click(screen.getByRole("button", { name: "View instructions for Old pack" }));
    expect(await screen.findByText(/delete this skill, add the ZIP again, then turn the skill back on/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Download Old pack" }));
    await waitFor(() => expect(saved.names).toEqual(["old-pack.md"]));
    expect(await saved.blobs[0]?.text()).toBe("# Old pack");
  });

  it("explains a failed download", async () => {
    setAuthSession("token", user);
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [skill], next_cursor: null })),
      http.get("*/v1/skills/:skillId/download", () => jsonResponse({ type: "about:blank", title: "Lỗi", status: 500, detail: "", code: "internal", fields: [] }, 500)),
    );
    const toastError = vi.spyOn(toast, "error");
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Download Clear writing" }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("Unable to download skill", expect.anything()));
    expect(screen.getByRole("button", { name: "Download Clear writing" })).toBeEnabled();
  });
});
