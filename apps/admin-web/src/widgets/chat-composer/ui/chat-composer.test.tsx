import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { type ComponentProps, useState } from "react";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

import type { Agent } from "@/entities/agent";

import { ChatComposer } from "./chat-composer";

const baseAgent = {
  description: "", provider_id: "provider-1", model: "model", system_prompt: "", temperature: null,
  max_output_tokens: null, show_thinking: false, context_window_tokens: 128000, max_iterations: 8, timeout_seconds: 120, created_by: "user-1", archived_at: null,
  created_at: "2026-09-17T00:00:00Z", updated_at: "2026-09-17T00:00:00Z", readiness_error: null,
} satisfies Omit<Agent, "id" | "name" | "ready">;

const agents: Agent[] = [
  { ...baseAgent, id: "agent-1", name: "First assistant", ready: true },
  { ...baseAgent, id: "agent-2", name: "Second assistant", ready: false },
];

function renderComposer(overrides: Partial<ComponentProps<typeof ChatComposer>> = {}) {
  const { value: initialValue = "", onSubmit = vi.fn(), ...rest } = overrides;
  function Harness() {
    const [value, setValue] = useState(initialValue);
    return <ChatComposer
      placeholder="Enter your request…"
      disabled={false}
      busy={false}
      switching={false}
      agents={agents}
      agentId="agent-1"
      onAgentChange={vi.fn()}
      onStop={vi.fn()}
      {...rest}
      value={value}
      onChange={setValue}
      onSubmit={onSubmit}
    />;
  }
  render(<Harness />);
  return { onSubmit };
}

function composerLayout() {
  return screen.getByLabelText("Message").parentElement?.dataset.layout;
}

// jsdom lays nothing out and reports scrollHeight as 0, so the measurement the
// layout switch is built on never runs. This stands in for it with the property
// that makes the switch tricky in a browser: the input is wider once the
// controls drop to their own row, so the same text wraps at one width and fits
// at the other.
const lineHeight = 24;
const charsPerLine = { inline: 20, stacked: 45 };

function typeInto(value: string) {
  fireEvent.change(screen.getByLabelText("Message"), { target: { value } });
  return composerLayout();
}

beforeAll(() => {
  Object.defineProperty(HTMLTextAreaElement.prototype, "scrollHeight", {
    configurable: true,
    get(this: HTMLTextAreaElement) {
      const layout = this.parentElement?.dataset.layout === "stacked" ? "stacked" : "inline";
      const perLine = charsPerLine[layout];
      const lines = this.value.split("\n").reduce((total, line) => total + Math.max(1, Math.ceil(line.length / perLine)), 0);
      return lines * lineHeight;
    },
  });
});

afterAll(() => {
  Reflect.deleteProperty(HTMLTextAreaElement.prototype, "scrollHeight");
});

describe("ChatComposer", () => {
  it("keeps the input and the controls on one row until the text needs a second line", () => {
    renderComposer();
    expect(composerLayout()).toBe("inline");

    expect(typeInto("a".repeat(charsPerLine.inline))).toBe("inline");
    expect(typeInto("a".repeat(charsPerLine.inline + 1))).toBe("stacked");
  });

  it("stacks on an explicit line break even when the text is short", () => {
    renderComposer();
    expect(typeInto("line 1\nline 2")).toBe("stacked");
    expect(typeInto("line 1")).toBe("inline");
  });

  it("gives one layout per text, whether that text was typed up to or pasted and cut back down", () => {
    // Between the two widths lies a range where the wide layout reports "fits on
    // one line" for a text that wraps as soon as the box goes back to one row.
    // Typing into an empty box never lands in that range, but pasting jumps
    // straight over it, so the way back down is what exposes a switch that reads
    // the layout currently on screen instead of the one-row width.
    const lengths = Array.from({ length: charsPerLine.stacked + 10 }, (_, index) => index + 1);

    renderComposer();
    const growing = lengths.map((length) => typeInto("a".repeat(length)));
    cleanup();

    renderComposer();
    const shrinking = [...lengths].reverse().map((length) => typeInto("a".repeat(length)));

    expect(shrinking.reverse()).toEqual(growing);
    expect(growing[0]).toBe("inline");
    expect(growing.at(-1)).toBe("stacked");
  });

  it("sends on Enter and adds a line on Shift+Enter", () => {
    const { onSubmit } = renderComposer({ value: "Hello" });
    const input = screen.getByLabelText("Message");

    fireEvent.keyDown(input, { key: "Enter", shiftKey: true });
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.keyDown(input, { key: "Enter" });
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("offers the assistant picker and refuses to send an empty message", () => {
    renderComposer();
    expect(screen.getByRole("combobox", { name: "Choose assistant" })).toHaveTextContent("First assistant");
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "   " } });
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Ready" } });
    expect(screen.getByRole("button", { name: "Send" })).toBeEnabled();
  });

  it("swaps the send button for a stop button while a response is streaming", () => {
    renderComposer({ busy: true, value: "Hello" });
    expect(screen.queryByRole("button", { name: "Send" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Stop" })).toBeInTheDocument();
    expect(screen.getByLabelText("Message")).toBeDisabled();
  });
});
