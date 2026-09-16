import createClient from "openapi-fetch";

import { clearAuthSession, getAuthSession } from "./auth-token-store";
import type { paths } from "./generated/schema";
import { refreshSingleFlight } from "./refresh-single-flight";

const isAuthPath = (url: string) => new URL(url, window.location.origin).pathname.startsWith("/v1/auth/");

function withAccessToken(request: Request, token: string | null) {
  const headers = new Headers(request.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);
  return new Request(request, { headers });
}

async function authenticatedFetch(request: Request) {
  const retry = request.clone();
  const response = await fetch(withAccessToken(request, getAuthSession().accessToken));
  if (response.status !== 401 || isAuthPath(request.url)) return response;

  const refreshed = await refreshSingleFlight();
  if (refreshed) return fetch(withAccessToken(retry, refreshed.access_token));

  clearAuthSession();
  const current = `${window.location.pathname}${window.location.search}`;
  if (window.location.pathname !== "/login") {
    window.location.assign(`/login?returnTo=${encodeURIComponent(current)}`);
  }
  return response;
}

export const apiClient = createClient<paths>({
  baseUrl: typeof window === "undefined" ? "" : window.location.origin,
  credentials: "include",
  fetch: authenticatedFetch,
});
