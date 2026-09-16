import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";

import { AgentSkillsPanel } from "./agent-skills-panel";

const skill = { id: "20000000-0000-4000-8000-000000000002", name: "Clear writing", description: "Use plain language.", source_type: "markdown" as const, source_filename: "writing.md", checksum: "0".repeat(64), created_by: "10000000-0000-4000-8000-000000000001", created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };

function renderPanel(readOnly = false, agentId = "30000000-0000-4000-8000-000000000003") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}><MemoryRouter><AgentSkillsPanel agentId={agentId} readOnly={readOnly} /></MemoryRouter></QueryClientProvider>);
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

    view.rerender(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><MemoryRouter><AgentSkillsPanel agentId={secondAgentId} /></MemoryRouter></QueryClientProvider>);

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
});
