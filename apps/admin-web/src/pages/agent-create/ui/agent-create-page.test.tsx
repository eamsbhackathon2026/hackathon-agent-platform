import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { toast } from "sonner";
import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { AgentCreatePage } from "./agent-create-page";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const providerId = "10000000-0000-4000-8000-000000000001";
const agentId = "20000000-0000-4000-8000-000000000002";
const provider = { id: providerId, name: "Primary Gemini", kind: "gemini" as const, base_url: null, default_model: "gemini-2.5-flash", api_key_hint: "1234", status: "ok" as const, last_error: null, last_checked_at: null, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };

function LocationMarker() { const location = useLocation(); return <p>route:{location.pathname}{location.search}</p>; }

afterEach(() => { clearAuthSession(); vi.clearAllMocks(); });

describe("provider to agent flow", () => {
  it("returns with the provider preselected, saves the agent, and links to playground", async () => {
    setAuthSession("token", { id: "user-1", email: "admin@example.test", name: "Admin", role: "admin", status: "active", must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" });
    let submittedProviderId = "";
    server.use(
      http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [provider], next_cursor: null })),
      http.get("*/v1/providers/:providerId/models", () => jsonResponse({ items: [{ id: "gemini-2.5-flash", display_name: "Gemini 2.5 Flash" }], next_cursor: null })),
      http.post(apiUrl("/v1/agents"), async ({ request }) => { const body = await request.json() as { provider_id: string }; submittedProviderId = body.provider_id; return jsonResponse({ id: agentId, name: "New assistant", description: "", provider_id: providerId, model: "gemini-2.5-flash", system_prompt: "Help the user.", temperature: 0.5, max_output_tokens: 2048, max_iterations: 8, timeout_seconds: 120, status: "active", created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z", readiness: "ready", readiness_error: null }, 201); }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[`/agents/new?provider=${providerId}`]}><Routes><Route path="/agents/new" element={<AgentCreatePage />} /><Route path="*" element={<LocationMarker />} /></Routes></MemoryRouter></QueryClientProvider>);

    const providerSelect = await screen.findByLabelText("Model connection");
    expect(providerSelect).toHaveValue(providerId);
    fireEvent.change(screen.getByLabelText("Assistant name"), { target: { value: "New assistant" } });
    fireEvent.change(screen.getByLabelText("AI model"), { target: { value: "gemini-2.5-flash" } });
    fireEvent.change(screen.getByLabelText("Assistant instructions"), { target: { value: "Help the user." } });
    fireEvent.click(screen.getByRole("button", { name: "Save assistant" }));

    expect(await screen.findByText(`route:/agents/${agentId}`)).toBeInTheDocument();
    expect(submittedProviderId).toBe(providerId);
    const successCall = vi.mocked(toast.success).mock.calls.find(([message]) => message === "Assistant created");
    const action = successCall?.[1]?.action;
    if (!action || typeof action !== "object" || !("onClick" in action)) throw new Error("Missing playground toast action");
    expect(action.label).toBe("Open playground");
    action.onClick({} as never);
    await waitFor(() => expect(screen.getByText(`route:/playground?agent=${agentId}`)).toBeInTheDocument());
  });
});
