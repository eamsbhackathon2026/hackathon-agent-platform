import type { components } from "@/shared/api";

export type ProblemCode = components["schemas"]["ProblemCode"];

export type ProblemAction = {
  title: string;
  description: string;
  action?: { label: string; to?: string; retry?: true };
};

export const problemActions = {
  validation_failed: { title: "Some information is invalid", description: "Review the highlighted fields and try again." },
  unauthenticated: { title: "Unable to sign in", description: "The email or password is incorrect. Please try again." },
  forbidden: { title: "You do not have permission", description: "Contact an administrator if you need to perform this action." },
  not_found: { title: "Content not found", description: "It may have been deleted, or you may not have permission to view it." },
  conflict: { title: "Unable to save changes", description: "The data was updated elsewhere. Reload and try again.", action: { label: "Reload", retry: true } },
  rate_limited: { title: "Too many requests", description: "Wait a moment and try again.", action: { label: "Try again", retry: true } },
  provider_not_configured: { title: "The assistant has no model connection", description: "Add credentials to a model connection before using this assistant.", action: { label: "Add connection", to: "/connections" } },
  provider_auth_failed: { title: "The model connection was rejected", description: "Check the credentials for the model connection.", action: { label: "Test connection", to: "/connections" } },
  provider_unreachable: { title: "Unable to reach the model connection", description: "Check the connection address or try again later.", action: { label: "Try again", retry: true } },
  model_not_found: { title: "Model not found", description: "Choose a model that is available through this model connection.", action: { label: "Edit assistant", to: "/agents" } },
  tool_failed: { title: "The tool did not complete", description: "Review the tool configuration and try again.", action: { label: "Open tools", to: "/tools" } },
  max_iterations_reached: { title: "The assistant needs too many steps", description: "Shorten the request or increase the assistant's processing limit." },
  loop_detected: { title: "The assistant is repeating itself", description: "Rephrase your request or review the assistant instructions." },
  run_timeout: { title: "The activity took too long", description: "Try again with a shorter request.", action: { label: "Try again", retry: true } },
  run_cancelled: { title: "The activity was stopped", description: "You can send the request again when you are ready." },
  run_in_progress: { title: "Another activity is in progress", description: "Wait for the current activity to finish, then try again." },
  interrupted: { title: "The activity was interrupted", description: "The system stopped safely to avoid repeating an action.", action: { label: "Try again", retry: true } },
  context_limit_exceeded: { title: "This conversation is too large", description: "Increase the assistant's conversation capacity or start a new conversation.", action: { label: "Edit assistant", to: "/agents" } },
  idempotency_key_reused: { title: "Duplicate request key", description: "Create a new request key when the content changes." },
  not_implemented: { title: "Feature not available yet", description: "This feature will be available in a future update." },
  internal: { title: "Something went wrong", description: "Try again. If the problem continues, contact an administrator.", action: { label: "Try again", retry: true } },
} satisfies Record<ProblemCode, ProblemAction>;

export function problemToAction(code: string | null | undefined): ProblemAction {
  if (code && code in problemActions) return problemActions[code as ProblemCode];
  return { title: "Something went wrong", description: "The system could not complete the request.", action: { label: "Try again", retry: true } };
}
