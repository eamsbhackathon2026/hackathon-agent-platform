import { z } from "zod";

export const agentFormSchema = z.object({
  name: z.string().trim().min(1, "Enter an assistant name").max(200),
  description: z.string().trim().max(4_000),
  provider_id: z.string().uuid("Choose a model connection"),
  model: z.string().trim().min(1, "Choose or enter an AI model").max(200),
  system_prompt: z.string().max(100_000),
  temperature: z.number().min(0).max(2).nullable(),
  max_output_tokens: z.number().int().positive().nullable(),
  show_thinking: z.boolean(),
  context_window_tokens: z.number().int().min(8_192).max(2_000_000),
  max_iterations: z.number().int().min(1, "Minimum 1 step").max(25, "Maximum 25 steps"),
  timeout_seconds: z.number().int().min(10, "Minimum 10 seconds").max(600, "Maximum 600 seconds"),
}).superRefine((value, context) => {
  const safety = Math.max(Math.floor(value.context_window_tokens / 10), 512);
  const maximumOutput = value.context_window_tokens - safety - 1_024;
  if (value.max_output_tokens !== null && value.max_output_tokens > maximumOutput) {
    context.addIssue({ code: "custom", path: ["max_output_tokens"], message: "Reduce the response length to leave room for instructions, tools, and messages" });
  }
});

export type AgentFormValues = z.infer<typeof agentFormSchema>;
