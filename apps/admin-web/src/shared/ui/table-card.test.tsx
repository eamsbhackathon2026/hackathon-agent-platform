import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TableCard } from "./table-card";

describe("TableCard", () => {
  it("names the region after its title unless a label says otherwise", () => {
    const { rerender } = render(<TableCard title="Recent requests">Rows</TableCard>);

    expect(screen.getByRole("region", { name: "Recent requests" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Recent requests" })).toBeInTheDocument();

    rerender(<TableCard title="Workspace members" label="Member list">Rows</TableCard>);

    expect(screen.getByRole("region", { name: "Member list" })).toBeInTheDocument();
  });

  it("shows the description line only when there is one", () => {
    const { rerender } = render(<TableCard title="Recent requests" description="Showing 2 requests">Rows</TableCard>);

    expect(screen.getByText("Showing 2 requests")).toBeInTheDocument();

    rerender(<TableCard title="Recent requests">Rows</TableCard>);

    expect(screen.queryByText("Showing 2 requests")).not.toBeInTheDocument();
  });
});
