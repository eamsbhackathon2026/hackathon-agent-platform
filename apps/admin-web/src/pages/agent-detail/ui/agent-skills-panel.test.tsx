import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";

import { AgentToolDraftProvider } from "../model";
import { AgentSkillsPanel } from "./agent-skills-panel";
import { AgentToolsPanel } from "./agent-tools-panel";

const skill = { id: "20000000-0000-4000-8000-000000000002", name: "Clear writing", description: "Use plain language.", source_type: "markdown" as const, source_filename: "writing.md", checksum: "0".repeat(64), tool_refs: [], source_file_available: false, created_by: "10000000-0000-4000-8000-000000000001", created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };

const httpTool = (id: string, slug: string) => ({ id, kind: "http" as const, slug, display_name: `Tool ${slug}`, description: "", method: "GET" as const, url_template: `/${slug}`, connection_id: null, params: [], public_headers: {}, secret_header_names: [], timeout_seconds: 15, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" });

/** The skills panel reads the tool draft, so every render supplies an empty catalog. */
function mockToolCatalog(tools: ReturnType<typeof httpTool>[] = [], boundToolIds: string[] = []) {
  server.use(
    http.get(apiUrl("/v1/tools"), () => jsonResponse({ items: tools, next_cursor: null })),
    http.get(apiUrl("/v1/mcp-servers"), () => jsonResponse({ items: [], next_cursor: null })),
    http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [], next_cursor: null })),
    http.get("*/v1/agents/:agentId/tools", () => jsonResponse({ tool_ids: boundToolIds, mcp_server_ids: [] })),
  );
}

function panel(agentId: string, readOnly: boolean, withTools = false) {
  return <MemoryRouter><AgentToolDraftProvider agentId={agentId}><AgentSkillsPanel agentId={agentId} readOnly={readOnly} />{withTools ? <AgentToolsPanel /> : null}</AgentToolDraftProvider></MemoryRouter>;
}

function renderPanel(readOnly = false, agentId = "30000000-0000-4000-8000-000000000003", withTools = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  mockToolCatalog();
  return render(<QueryClientProvider client={client}>{panel(agentId, readOnly, withTools)}</QueryClientProvider>);
}

describe("AgentSkillsPanel", () => {
  it("replaces the complete selected skill list", async () => {
    let saved: unknown;
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [skill], next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: [] })),
      http.put("*/v1/agents/:agentId/skills", async ({ request }) => { saved = await request.json(); return jsonResponse({ skill_ids: [skill.id] }); }),
    );
    renderPanel();
    fireEvent.click(await screen.findByRole("checkbox", { name: "Clear writing" }));
    fireEvent.click(screen.getByRole("button", { name: "Save skill list" }));
    await waitFor(() => expect(saved).toEqual({ skill_ids: [skill.id] }));
  });

  it("shows selected skills without mutation controls in read-only mode", async () => {
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [skill], next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: [skill.id] })),
    );
    renderPanel(true);
    expect(await screen.findByRole("checkbox", { name: "Clear writing" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Save skill list" })).not.toBeInTheDocument();
  });

  it("locks the draft while a replacement request is pending", async () => {
    let completeRequest: (() => void) | undefined;
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [skill], next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: [] })),
      http.put("*/v1/agents/:agentId/skills", async () => {
        await new Promise<void>((resolve) => { completeRequest = resolve; });
        return jsonResponse({ skill_ids: [skill.id] });
      }),
    );
    renderPanel();
    const checkbox = await screen.findByRole("checkbox", { name: "Clear writing" });
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByRole("button", { name: "Save skill list" }));

    expect(await screen.findByRole("button", { name: "Saving…" })).toBeDisabled();
    expect(checkbox).toBeDisabled();
    completeRequest?.();
    await waitFor(() => expect(screen.getByRole("button", { name: "Save skill list" })).toBeDisabled());
  });

  it("does not carry an unsaved draft to another assistant", async () => {
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [skill], next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: [] })),
    );
    const firstAgentId = "30000000-0000-4000-8000-000000000003";
    const secondAgentId = "30000000-0000-4000-8000-000000000004";
    const view = renderPanel(false, firstAgentId);
    const checkbox = await screen.findByRole("checkbox", { name: "Clear writing" });
    fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();

    view.rerender(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{panel(secondAgentId, false)}</QueryClientProvider>);

    await waitFor(() => expect(screen.getByRole("checkbox", { name: "Clear writing" })).not.toBeChecked());
    expect(screen.getByRole("button", { name: "Save skill list" })).toBeDisabled();
  });

  it("prevents selecting a twenty-first skill while allowing deselection", async () => {
    const manySkills = Array.from({ length: 21 }, (_, index) => ({
      ...skill,
      id: `20000000-0000-4000-8000-${String(index + 10).padStart(12, "0")}`,
      name: `Skill ${index + 1}`,
    }));
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: manySkills, next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: manySkills.slice(0, 20).map((item) => item.id) })),
    );
    renderPanel();

    expect(await screen.findByText("20/20 selected")).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Skill 21" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "Skill 1" })).not.toBeDisabled();
  });
  it("offers the tools a newly selected skill declares and ticks them on confirmation", async () => {
    const insights = httpTool("tool-1", "get_insights");
    const finance = { ...skill, tool_refs: ["get_insights"] };
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [finance], next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: [] })),
    );
    mockToolCatalog([insights]);
    render(<QueryClientProvider client={client}>{panel("30000000-0000-4000-8000-000000000003", false, true)}</QueryClientProvider>);

    // An open Radix dialog marks the rest of the page aria-hidden, so the tool checkbox
    // is read before the dialog opens rather than while it is up.
    expect(await screen.findByRole("checkbox", { name: /Tool get_insights/ })).not.toBeChecked();
    fireEvent.click(await screen.findByRole("checkbox", { name: "Clear writing" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Tool get_insights")).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "Turn on 1" }));
    await waitFor(() => expect(screen.getByRole("checkbox", { name: /Tool get_insights/ })).toBeChecked());
  });

  it("keeps the skill ticked when the tool offer is declined", async () => {
    const insights = httpTool("tool-1", "get_insights");
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [{ ...skill, tool_refs: ["get_insights"] }], next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: [] })),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockToolCatalog([insights]);
    render(<QueryClientProvider client={client}>{panel("30000000-0000-4000-8000-000000000003", false, true)}</QueryClientProvider>);

    const checkbox = await screen.findByRole("checkbox", { name: "Clear writing" });
    fireEvent.click(checkbox);
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Not now" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(checkbox).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Tool get_insights/ })).not.toBeChecked();
    expect(await screen.findByText("Missing 1 tool")).toBeInTheDocument();
  });

  it("warns about a reference no tool in the workspace matches", async () => {
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [{ ...skill, tool_refs: ["nothing_here"] }], next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: [] })),
    );
    renderPanel();
    fireEvent.click(await screen.findByRole("checkbox", { name: "Clear writing" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/\$nothing_here/)).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Turn on 0" })).toBeDisabled();
  });

  it("offers to turn off a tool no remaining skill needs", async () => {
    const insights = httpTool("tool-1", "get_insights");
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [{ ...skill, tool_refs: ["get_insights"] }], next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: [skill.id] })),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockToolCatalog([insights], [insights.id]);
    render(<QueryClientProvider client={client}>{panel("30000000-0000-4000-8000-000000000003", false, true)}</QueryClientProvider>);

    expect(await screen.findByRole("checkbox", { name: /Tool get_insights/ })).toBeChecked();
    fireEvent.click(await screen.findByRole("checkbox", { name: "Clear writing" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Turn off 1" }));

    await waitFor(() => expect(screen.getByRole("checkbox", { name: /Tool get_insights/ })).not.toBeChecked());
  });

  it("leaves a tool on when another selected skill still needs it", async () => {
    const insights = httpTool("tool-1", "get_insights");
    const first = { ...skill, tool_refs: ["get_insights"] };
    const second = { ...skill, id: "20000000-0000-4000-8000-000000000009", name: "Second skill", tool_refs: ["get_insights"] };
    server.use(
      http.get(apiUrl("/v1/skills"), () => jsonResponse({ items: [first, second], next_cursor: null })),
      http.get("*/v1/agents/:agentId/skills", () => jsonResponse({ skill_ids: [first.id, second.id] })),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockToolCatalog([insights], [insights.id]);
    render(<QueryClientProvider client={client}>{panel("30000000-0000-4000-8000-000000000003", false, true)}</QueryClientProvider>);

    fireEvent.click(await screen.findByRole("checkbox", { name: "Clear writing" }));
    await waitFor(() => expect(screen.getByRole("checkbox", { name: "Clear writing" })).not.toBeChecked());
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /Tool get_insights/ })).toBeChecked();
  });
});
