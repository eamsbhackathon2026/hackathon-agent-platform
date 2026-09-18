import { describe, expect, it } from "vitest";
import { agentFormSchema } from "./agent-form-schema";

const valid = { name: "Orders", description: "", provider_id: "123e4567-e89b-12d3-a456-426614174000", model: "fast", system_prompt: "", temperature: 0.5, max_output_tokens: 1024, show_thinking: false, context_window_tokens: 32768, max_iterations: 8, timeout_seconds: 120 };
describe("agentFormSchema", () => {
  it("accepts documented boundaries", () => expect(agentFormSchema.safeParse({ ...valid, max_iterations: 25, timeout_seconds: 600 }).success).toBe(true));
  it("rejects too many steps and too short timeout", () => {
    expect(agentFormSchema.safeParse({ ...valid, max_iterations: 26 }).success).toBe(false);
    expect(agentFormSchema.safeParse({ ...valid, timeout_seconds: 9 }).success).toBe(false);
  });
  it("keeps the response inside the conversation capacity", () => {
    expect(agentFormSchema.safeParse({ ...valid, context_window_tokens: 8_192, max_output_tokens: 8_192 }).success).toBe(false);
    expect(agentFormSchema.safeParse({ ...valid, context_window_tokens: 8_192, max_output_tokens: 6_350 }).success).toBe(false);
    expect(agentFormSchema.safeParse({ ...valid, context_window_tokens: 8_192, max_output_tokens: 6_349 }).success).toBe(true);
  });
  it("requires an explicit choice about narrating the thinking", () => {
    // Omitting it makes the form send undefined, which the server reads as "leave
    // unchanged", so the value is always present; the off default lives in the form.
    const { show_thinking, ...missing } = valid;
    void show_thinking;
    expect(agentFormSchema.safeParse(missing).success).toBe(false);
    expect(agentFormSchema.safeParse({ ...valid, show_thinking: true }).success).toBe(true);
  });
  it("matches the OpenAPI text limits", () => {
    expect(agentFormSchema.safeParse({ ...valid, name: "a".repeat(200), description: "a".repeat(4_000), model: "a".repeat(200), system_prompt: "a".repeat(100_000) }).success).toBe(true);
    expect(agentFormSchema.safeParse({ ...valid, name: "a".repeat(201) }).success).toBe(false);
  });
});
