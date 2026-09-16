import { afterEach, describe, expect, it, vi } from "vitest";

import { clearAuthSession, setAuthSession } from "@/shared/api";

import { importSkill } from "./import-skill";

afterEach(() => { clearAuthSession(); vi.restoreAllMocks(); });

describe("importSkill", () => {
  it("sends the selected file as authenticated multipart data", async () => {
    setAuthSession("access-token", { id: "10000000-0000-4000-8000-000000000001", email: "admin@example.test", name: "Admin", role: "admin", status: "active", must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" });
    let contentType = "";
    let authorization = "";
    let requestBody = "";
    const append = vi.spyOn(FormData.prototype, "append");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const request = input as Request;
      contentType = request.headers.get("Content-Type") ?? "";
      authorization = request.headers.get("Authorization") ?? "";
      requestBody = await request.text();
      return new Response(JSON.stringify({ id: "20000000-0000-4000-8000-000000000002", name: "Writing", description: "", source_type: "markdown", source_filename: "writing.md", content: "# Writing", checksum: "0".repeat(64), created_by: "10000000-0000-4000-8000-000000000001", created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" }), { status: 201, headers: { "Content-Type": "application/json" } });
    });

    const file = new File(["# Writing"], "writing.md", { type: "text/markdown" });
    const result = await importSkill(file);

    expect(result.name).toBe("Writing");
    expect(contentType).toMatch(/^multipart\/form-data; boundary=/);
    expect(authorization).toBe("Bearer access-token");
    expect(append).toHaveBeenCalledWith("file", file, "writing.md");
    expect(requestBody).toContain('name="file"');
    expect(requestBody).toContain("form-data");
  });
});
