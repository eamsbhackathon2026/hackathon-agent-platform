import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";
import { server } from "@/test/msw-server";
import { jsonResponse } from "@/test/typed-handlers";
import { AgentDetailPage } from "./agent-detail-page";

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={["/agents/agent-1"]}><Routes><Route path="/agents/:agentId" element={<AgentDetailPage />} /></Routes></MemoryRouter></QueryClientProvider>);
}

describe("AgentDetailPage", () => {
  it("distinguishes not found from a temporary API failure", async () => {
    server.use(http.get("*/v1/agents/:agentId", () => jsonResponse({ status: 404 }, 404)));
    renderPage();
    expect(await screen.findByText(/This assistant was not found/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
  });
  it("offers retry for a temporary API failure", async () => {
    let failed = true;
    server.use(http.get("*/v1/agents/:agentId", () => failed ? jsonResponse({ status: 500 }, 500) : jsonResponse({ status: 404 }, 404)));
    renderPage();
    expect(await screen.findByText(/Unable to load the assistant/)).toBeInTheDocument();
    failed = false; fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText(/This assistant was not found/)).toBeInTheDocument();
  });
});
