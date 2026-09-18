import { describe, expect, it } from "vitest";

import type { McpServer } from "@/entities/mcp-server";
import type { HttpTool } from "@/entities/tool";

import { missingSkillTools, resolveSkillTools, type SkillToolCatalog } from "./resolve-skill-tools";

const tool = (id: string, slug: string, connectionId: string | null = null) =>
  ({ id, kind: "http", slug, display_name: `Tool ${slug}`, step_label: "", description: "", method: "GET", url_template: `/${slug}`, connection_id: connectionId, params: [], public_headers: {}, secret_header_names: [], timeout_seconds: 15, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" }) as HttpTool;

const mcpServer = (id: string, slug: string, toolNames: string[]) =>
  ({ id, slug, display_name: `Server ${slug}`, url: "https://example.test/mcp", allowed_tools: null, secret_header_names: [], tools: toolNames.map((name) => ({ name, description: "", input_schema: {} })), status: "ready", last_error: null, last_synced_at: null, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" }) as unknown as McpServer;

const catalog: SkillToolCatalog = {
  tools: [tool("t1", "get_insights", "conn-a"), tool("t2", "ping")],
  servers: [mcpServer("s1", "finance", ["get_portfolio", "list_products"])],
  connectionNames: new Map([["conn-a", "Giao dịch"]]),
};

describe("resolveSkillTools", () => {
  it("matches an HTTP reference by slug and names the connection it belongs to", () => {
    const { resolved, unresolved } = resolveSkillTools(["get_insights"], catalog);
    expect(unresolved).toEqual([]);
    expect(resolved).toEqual([{ kind: "tool", id: "t1", ref: "get_insights", label: "Tool get_insights", groupLabel: "Giao dịch" }]);
  });

  it("puts a direct-URL tool in its own group", () => {
    expect(resolveSkillTools(["ping"], catalog).resolved[0]?.groupLabel).toBe("Direct URL");
  });

  it("resolves an MCP reference to the server that carries the tool", () => {
    const { resolved } = resolveSkillTools(["finance.get_portfolio"], catalog);
    expect(resolved).toEqual([{ kind: "server", id: "s1", ref: "finance.get_portfolio", label: "Server finance", groupLabel: "Tool servers" }]);
  });

  it("collapses two references into the same server", () => {
    const { resolved } = resolveSkillTools(["finance.get_portfolio", "finance.list_products"], catalog);
    expect(resolved).toHaveLength(1);
    expect(resolved[0]?.id).toBe("s1");
  });

  it("reports a reference nothing matches instead of dropping it", () => {
    const { resolved, unresolved } = resolveSkillTools(["nothing_here", "finance.missing_tool", "unknown.tool"], catalog);
    expect(resolved).toEqual([]);
    expect(unresolved).toEqual(["nothing_here", "finance.missing_tool", "unknown.tool"]);
  });

  it("matches case-sensitively, because slugs are", () => {
    expect(resolveSkillTools(["GET_INSIGHTS"], catalog).unresolved).toEqual(["GET_INSIGHTS"]);
  });
});

describe("missingSkillTools", () => {
  it("keeps only what the current selection does not already have", () => {
    const resolution = resolveSkillTools(["get_insights", "ping", "finance.get_portfolio"], catalog);
    const missing = missingSkillTools(resolution, { toolIds: ["t1"], serverIds: ["s1"] });
    expect(missing.map((item) => item.ref)).toEqual(["ping"]);
  });

  it("returns nothing when every tool is already on", () => {
    const resolution = resolveSkillTools(["get_insights"], catalog);
    expect(missingSkillTools(resolution, { toolIds: ["t1"], serverIds: [] })).toEqual([]);
  });
});
