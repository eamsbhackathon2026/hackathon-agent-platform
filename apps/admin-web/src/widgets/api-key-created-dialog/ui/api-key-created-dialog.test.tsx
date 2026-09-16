import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { ApiKeyCreated } from "@/entities/api-key";

import { ApiKeyCreatedDialog } from "./api-key-created-dialog";

const created: ApiKeyCreated = {
  api_key: {
    id: "key-1",
    name: "Website",
    prefix: "demo1234",
    scopes: ["runs:write"],
    created_by: "user-1",
    last_used_at: null,
    revoked_at: null,
    created_at: "2026-09-15T00:00:00Z",
  },
  key: `apk_${"k".repeat(72)}`,
  webhook_secret: `whsec_${"s".repeat(64)}`,
};

describe("ApiKeyCreatedDialog", () => {
  it("keeps long secrets and snippets inside the responsive dialog", () => {
    render(
      <MemoryRouter>
        <ApiKeyCreatedDialog created={created} agentId="agent-1" onClose={() => undefined} />
      </MemoryRouter>,
    );

    const dialog = screen.getByRole("dialog", { name: "Save your access key now" });
    expect(dialog).toHaveClass("max-h-[calc(100dvh-2rem)]", "overflow-y-auto");

    const accessKey = within(dialog).getByRole("textbox", { name: "Access key" });
    const signingSecret = within(dialog).getByRole("textbox", { name: "Delivery signing secret" });
    for (const input of [accessKey, signingSecret]) {
      expect(input).toHaveClass("min-w-0", "flex-1");
      expect(input.parentElement).toHaveClass("min-w-0");
    }

    const tabList = within(dialog).getByRole("tablist");
    expect(tabList).toHaveClass("max-w-full", "justify-start", "overflow-x-auto");
    for (const tab of within(tabList).getAllByRole("tab")) {
      expect(tab).toHaveClass("shrink-0");
    }

    const codeBlock = dialog.querySelector("pre");
    expect(codeBlock).toHaveClass("min-w-0", "w-full", "max-w-full", "overflow-x-auto");
  });
});
