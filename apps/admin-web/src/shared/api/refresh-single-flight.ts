import { clearAuthSession, setAuthSession } from "./auth-token-store";
import type { components } from "./generated/schema";

type TokenResponse = components["schemas"]["TokenResponse"];
type RefreshOperation = () => Promise<TokenResponse | null>;

let currentRefresh: Promise<TokenResponse | null> | null = null;

async function requestRefresh(): Promise<TokenResponse | null> {
  const response = await fetch(new URL("/v1/auth/refresh", window.location.origin), {
    method: "POST",
    credentials: "include",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) return null;
  return (await response.json()) as TokenResponse;
}

export function refreshSingleFlight(operation: RefreshOperation = requestRefresh) {
  if (currentRefresh) return currentRefresh;
  currentRefresh = operation()
    .then((result) => {
      if (result) setAuthSession(result.access_token, result.me.user);
      else clearAuthSession();
      return result;
    })
    .catch(() => {
      clearAuthSession();
      return null;
    })
    .finally(() => {
      currentRefresh = null;
    });
  return currentRefresh;
}

export function resetRefreshForTests() {
  currentRefresh = null;
}
