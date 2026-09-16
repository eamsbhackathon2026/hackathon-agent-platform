import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CardContent } from "./card";

describe("CardContent", () => {
  it("allows callers to add top padding at every breakpoint", () => {
    render(<CardContent className="pt-6">Table content</CardContent>);

    const content = screen.getByText("Table content");
    expect(content).toHaveClass("pt-6");
    expect(content).not.toHaveClass("sm:pt-0");
  });
});
