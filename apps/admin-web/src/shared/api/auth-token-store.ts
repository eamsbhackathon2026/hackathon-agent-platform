import { useSyncExternalStore } from "react";

import type { components } from "./generated/schema";

export type SessionUser = components["schemas"]["User"];

type AuthSession = {
  accessToken: string | null;
  user: SessionUser | null;
};

let session: AuthSession = { accessToken: null, user: null };
const listeners = new Set<() => void>();

export function getAuthSession() {
  return session;
}

export function setAuthSession(accessToken: string, user: SessionUser) {
  session = { accessToken, user };
  listeners.forEach((listener) => listener());
}

export function clearAuthSession() {
  session = { accessToken: null, user: null };
  listeners.forEach((listener) => listener());
}

export function updateSessionUser(user: SessionUser) {
  session = { ...session, user };
  listeners.forEach((listener) => listener());
}

export function useAuthSession() {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    getAuthSession,
    getAuthSession,
  );
}
