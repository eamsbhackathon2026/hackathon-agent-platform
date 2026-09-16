import { HttpResponse, http } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";

import { clearAuthSession, getAuthSession, setAuthSession } from "../auth-token-store";
import { resetRefreshForTests } from "../refresh-single-flight";
import { server } from "@/test/msw-server";
import { jsonResponse } from "@/test/typed-handlers";
import { postSse } from "./post-sse";

const user = { id: "user-1", email: "member@example.test", name: "Member", role: "member" as const, status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };

afterEach(() => { clearAuthSession(); resetRefreshForTests(); });

describe("postSse", () => {
  it("keeps the Problem response for an HTTP failure before streaming", async () => {
    server.use(http.post("*/v1/test-stream", () => jsonResponse({ type: "about:blank", title: "Not ready", status: 400, detail: "The assistant has no AI connection.", code: "provider_not_configured", fields: [] }, 400)));
    await expect(postSse("/v1/test-stream", {})).rejects.toMatchObject({
      status: 400, problem: { code: "provider_not_configured", detail: "The assistant has no AI connection." },
    });
  });

  it("refreshes once and retries with the new access token", async () => {
    setAuthSession("old-token", user);
    const authorizations = vi.fn();
    server.use(
      http.post("*/v1/test-stream", ({ request }) => {
        const token = request.headers.get("Authorization");
        authorizations(token);
        return token === "Bearer new-token"
          ? new HttpResponse(new ReadableStream({ start: (controller) => controller.close() }), { headers: { "Content-Type": "text/event-stream" } })
          : jsonResponse({ type: "about:blank", title: "Hết hạn", status: 401, detail: "", code: "unauthenticated", fields: [] }, 401);
      }),
      http.post("*/v1/auth/refresh", () => jsonResponse({ access_token: "new-token", token_type: "Bearer", expires_in: 900, me: { user } })),
    );
    await expect(postSse("/v1/test-stream", {})).resolves.toBeInstanceOf(ReadableStream);
    expect(authorizations.mock.calls.flat()).toEqual(["Bearer old-token", "Bearer new-token"]);
  });

  it("clears the session and exposes an authentication action when refresh fails", async () => {
    setAuthSession("expired", user);
    server.use(
      http.post("*/v1/test-stream", () => jsonResponse({ type: "about:blank", title: "Session expired", status: 401, detail: "Sign in again.", code: "unauthenticated", fields: [] }, 401)),
      http.post("*/v1/auth/refresh", () => HttpResponse.json({}, { status: 401 })),
    );
    await expect(postSse("/v1/test-stream", {})).rejects.toMatchObject({ status: 401, problem: { code: "unauthenticated" } });
    expect(getAuthSession().user).toBeNull();
  });
});
