import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import type { ToolImportItem } from "../api/tool-transfer";
import { importDecisions } from "../lib/import-decisions";
import { ToolImportDialog } from "./tool-import-dialog";

const admin = { id: "user-1", email: "admin@example.test", name: "Admin", role: "admin" as const, status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };
const bundle = { format: "agent-platform.tools" as const, version: 1 as const, connections: [], tools: [] };
const item = (overrides: Partial<ToolImportItem>): ToolImportItem => ({ kind: "tool", slug: "orders", display_name: "Orders", status: "new", fields: [], ...overrides });

function renderDialog(onImported = vi.fn().mockResolvedValue(undefined)) {
  render(<MemoryRouter><ToolImportDialog open onOpenChange={() => undefined} onImported={onImported} /></MemoryRouter>);
  return onImported;
}

function chooseFile(content: string) {
  const file = new File([content], "tools.json", { type: "application/json" });
  fireEvent.change(screen.getByLabelText("Tools file"), { target: { files: [file] } });
  fireEvent.click(screen.getByRole("button", { name: "Review file" }));
}

afterEach(() => { clearAuthSession(); vi.restoreAllMocks(); });

describe("importDecisions", () => {
  it("skips invalid and undecided existing items but never a new item sharing a slug", () => {
    const items = [item({}), item({ status: "invalid", fields: [{ field: "slug", message: "repeated" }] }), item({ slug: "broken", status: "invalid" }), item({ kind: "connection", slug: "crm", status: "conflict" }), item({ slug: "billing", status: "conflict" })];
    expect(importDecisions(items, { "tool:billing": "overwrite" })).toEqual([
      { kind: "connection", slug: "crm", action: "skip" },
      { kind: "tool", slug: "billing", action: "overwrite" },
      { kind: "tool", slug: "broken", action: "skip" },
    ]);
  });
});

describe("ToolImportDialog", () => {
  it("explains a file that is not an exported tools file without calling the server", async () => {
    setAuthSession("token", admin); const preview = vi.fn(); server.use(http.post(apiUrl("/v1/tools/import/preview"), () => { preview(); return jsonResponse({ items: [] }); }));
    renderDialog(); chooseFile("not json");
    expect(await screen.findByRole("alert")).toHaveTextContent("not valid JSON");
    expect(preview).not.toHaveBeenCalled();
  });

  it("previews items, keeps existing ones by default, and imports the chosen replacements", async () => {
    setAuthSession("token", admin);
    let sent: unknown;
    server.use(
      http.post(apiUrl("/v1/tools/import/preview"), () => jsonResponse({ items: [item({ status: "conflict" }), item({ slug: "broken", display_name: "Broken", status: "invalid", fields: [{ field: "url_template", message: "Address is not allowed." }] })] })),
      http.post(apiUrl("/v1/tools/import"), async ({ request }) => { sent = await request.json(); return jsonResponse({ created: 0, overwritten: 1, skipped: 1, needs_secrets: [{ kind: "tool", id: "tool-1", slug: "orders", display_name: "Orders", header_names: ["Authorization"] }] }); }),
    );
    const onImported = renderDialog(); chooseFile(JSON.stringify(bundle));

    expect(await screen.findByText("Already exists")).toBeInTheDocument();
    expect(screen.getByText("Address is not allowed.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Import 0 items" })).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Choice for Orders"), { target: { value: "overwrite" } });
    fireEvent.click(screen.getByRole("button", { name: "Import 1 item" }));

    expect(await screen.findByText("Imported: 0 added, 1 replaced, 1 skipped.")).toBeInTheDocument();
    expect(sent).toEqual({ bundle, decisions: [{ kind: "tool", slug: "orders", action: "overwrite" }, { kind: "tool", slug: "broken", action: "skip" }] });
    expect(screen.getByRole("link", { name: "Orders" })).toHaveAttribute("href", "/tools/tool-1/edit");
    await waitFor(() => expect(onImported).toHaveBeenCalled());
  });

  it("shows the server's reasons when nothing could be imported", async () => {
    setAuthSession("token", admin);
    server.use(
      http.post(apiUrl("/v1/tools/import/preview"), () => jsonResponse({ items: [item({})] })),
      http.post(apiUrl("/v1/tools/import"), () => jsonResponse({ type: "about:blank", title: "Invalid", status: 400, code: "validation_failed", detail: "Some items could not be imported.", fields: [{ field: "tools.orders.url_template", message: "Address is not allowed." }] }, 400)),
    );
    const onImported = renderDialog(); chooseFile(JSON.stringify(bundle));
    fireEvent.click(await screen.findByRole("button", { name: "Import 1 item" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Some items could not be imported. Address is not allowed.");
    expect(onImported).not.toHaveBeenCalled();
  });
});
