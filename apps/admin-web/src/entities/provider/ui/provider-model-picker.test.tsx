import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ProviderModelPicker } from "./provider-model-picker";

const models = [
  { id: "z-ai/glm-5.2-hackathon", display_name: "GLM 5.2 Hackathon" },
  { id: "qwen/qwen3.6-flash", display_name: "Qwen 3.6 Flash" },
  { id: "other/model", display_name: "Other" },
];

describe("ProviderModelPicker", () => {
  it("shows model logos, selection, manual entry, and validation", () => {
    const onChange = vi.fn();
    render(<ProviderModelPicker error="Choose a model" label="Default model" models={models} value={models[0]!.id} onChange={onChange} />);
    expect(screen.getByLabelText("Default model")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("img", { name: "Z.ai" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Qwen logo" })).toBeInTheDocument();
    expect(screen.getByText("AI")).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /GLM 5.2 Hackathon/ })).toHaveAttribute("aria-selected", "true");
    fireEvent.click(screen.getByRole("option", { name: /Qwen 3.6 Flash/ }));
    fireEvent.change(screen.getByLabelText("Default model"), { target: { value: "manual/model" } });
    expect(onChange).toHaveBeenNthCalledWith(1, "qwen/qwen3.6-flash");
    expect(onChange).toHaveBeenNthCalledWith(2, "manual/model");
    expect(screen.getByText("Choose a model")).toBeInTheDocument();
  });

  it("supports a free-form model without suggestions", () => {
    render(<ProviderModelPicker models={[]} value="manual/model" onChange={() => undefined} />);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(screen.getByLabelText("AI model")).toHaveValue("manual/model");
  });

  it("offers a compact searchable combobox with custom model fallback", () => {
    const onChange = vi.fn();
    render(<ProviderModelPicker display="combobox" models={models} value={models[0]!.id} onChange={onChange} />);
    const trigger = screen.getByRole("combobox", { name: "AI model" });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    const search = screen.getByPlaceholderText("Search models...");
    expect(search).toHaveClass("rounded-none", "focus-visible:outline-none", "focus-visible:ring-0");
    expect(search).toHaveStyle({ outline: "none", boxShadow: "none" });
    fireEvent.change(search, { target: { value: "qwen" } });
    fireEvent.click(screen.getByText("Qwen 3.6 Flash"));
    expect(onChange).toHaveBeenLastCalledWith("qwen/qwen3.6-flash");
    fireEvent.click(trigger);
    fireEvent.change(screen.getByPlaceholderText("Search models..."), { target: { value: "future/model" } });
    fireEvent.click(screen.getByText("Use custom model"));
    expect(onChange).toHaveBeenLastCalledWith("future/model");
  });
});
