import { clearAuthSession, getAuthSession } from "../auth-token-store";
import type { components } from "../generated/schema";
import { refreshSingleFlight } from "../refresh-single-flight";

type PostSseOptions = {
  signal?: AbortSignal;
};

export class SseHttpError extends Error {
  constructor(public readonly status: number, public readonly problem: components["schemas"]["Problem"] | null) {
    super(problem?.detail || problem?.title || `SSE request failed (${status})`);
    this.name = "SseHttpError";
  }
}

function request(path: string, body: unknown, token: string | null, signal?: AbortSignal) {
  const headers = new Headers({ Accept: "text/event-stream", "Content-Type": "application/json" });
  if (token) headers.set("Authorization", `Bearer ${token}`);
  return fetch(path, { method: "POST", headers, body: JSON.stringify(body), credentials: "include", signal: signal ?? null });
}

export async function postSse(path: string, body: unknown, options: PostSseOptions = {}) {
  let response = await request(path, body, getAuthSession().accessToken, options.signal);
  if (response.status === 401) {
    const refreshed = await refreshSingleFlight();
    if (refreshed) response = await request(path, body, refreshed.access_token, options.signal);
    else clearAuthSession();
  }
  if (!response.ok) {
    const problem = await response.clone().json().catch(() => null) as components["schemas"]["Problem"] | null;
    throw new SseHttpError(response.status, problem);
  }
  if (!response.body) throw new Error("SSE response does not contain a stream");
  return response.body;
}
