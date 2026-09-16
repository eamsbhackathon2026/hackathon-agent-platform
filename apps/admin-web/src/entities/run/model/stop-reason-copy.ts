import { problemToAction, type ProblemAction } from "@/shared/lib/problem-to-action";

const stopReasonCopy = {
  unauthenticated: {
    title: "Your session has expired",
    description: "Sign in again to continue the conversation.",
    action: { label: "Sign in", to: "/login" },
  },
  loop_detected: {
    title: "The assistant stopped after repeating the same action",
    description: "Review the enabled tools and the assistant instructions.",
    action: { label: "View tools", to: "/tools" },
  },
  max_iterations_reached: {
    title: "This request needs more steps than allowed",
    description: "Increase the step limit if this request genuinely needs more actions.",
    action: { label: "Increase step limit", to: "/agents" },
  },
  run_timeout: {
    title: "The activity timed out",
    description: "Increase the timeout or shorten the request.",
    action: { label: "Edit assistant", to: "/agents" },
  },
  provider_auth_failed: {
    title: "The model connection was rejected",
    description: "Check the credentials for this model connection.",
    action: { label: "Edit model connection", to: "/connections" },
  },
  provider_unreachable: {
    title: "The model connection could not be reached",
    description: "Check the connection address or try again later.",
    action: { label: "Edit model connection", to: "/connections" },
  },
  interrupted: {
    title: "The server restarted while processing",
    description: "The request was stopped to avoid repeating an unintended action.",
    action: { label: "Send again", retry: true },
  },
  run_cancelled: {
    title: "The activity was stopped",
    description: "You can send the request again when you are ready.",
    action: { label: "Send again", retry: true },
  },
} satisfies Record<string, ProblemAction>;

export function stopReasonToAction(code: string | null | undefined, agentId?: string): ProblemAction | null {
  if (!code) return null;
  const copy = stopReasonCopy[code as keyof typeof stopReasonCopy];
  if (copy && agentId && (code === "max_iterations_reached" || code === "run_timeout")) {
    return { ...copy, action: { ...copy.action, to: `/agents/${agentId}#advanced` } };
  }
  return copy ?? problemToAction(code);
}
