import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { ApiAccessGuide } from "./api-access-guide";

describe("ApiAccessGuide", () => {
  it("explains the complete API integration flow and exposes the next actions", () => {
    const onCreateKey = vi.fn();
    render(
      <MemoryRouter>
        <ApiAccessGuide onCreateKey={onCreateKey} />
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: "Connect your application" })).toBeInTheDocument();
    const showGuide = screen.getByRole("button", { name: "View integration guide" });
    expect(showGuide).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText(/X-API-Key/)).not.toBeInTheDocument();

    fireEvent.click(showGuide);
    expect(screen.getByRole("button", { name: "Hide integration guide" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
    expect(screen.getByText(/X-API-Key/)).toBeInTheDocument();
    expect(screen.getByText(/\/v1\/agents\/\$AGENT_ID\/runs/)).toBeInTheDocument();
    expect(screen.getByText("Immediate")).toBeInTheDocument();
    expect(screen.getByText("Live stream")).toBeInTheDocument();
    expect(screen.getByText("Background")).toBeInTheDocument();
    expect(screen.getByText(/Polling the Location header requires View status and results/)).toBeInTheDocument();
    const securityNote = screen.getByText(/verify the signature against the raw body before parsing/);
    expect(securityNote).toHaveTextContent(/reject timestamps more than five minutes apart/);
    expect(securityNote).toHaveTextContent(/atomically deduplicate each webhook ID/);

    fireEvent.click(screen.getByRole("button", { name: "Create access key" }));
    expect(onCreateKey).toHaveBeenCalledOnce();
    expect(screen.getByRole("link", { name: "View activity" })).toHaveAttribute("href", "/activity");

    fireEvent.click(screen.getByRole("button", { name: "Hide integration guide" }));
    expect(screen.queryByText(/X-API-Key/)).not.toBeInTheDocument();
  });
});
