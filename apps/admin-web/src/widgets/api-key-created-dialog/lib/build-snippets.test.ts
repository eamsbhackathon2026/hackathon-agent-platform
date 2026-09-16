import { describe, expect, it } from "vitest";
import { buildSnippets } from "./build-snippets";

describe("buildSnippets", () => {
  it("builds executable sync, stream and async examples", () => {
    const snippets = buildSnippets({ baseUrl: "https://api.example.com/", apiKey: "apk_demo_secret", agentId: "agent-7" });
    expect(snippets.sync).toContain("agent-7");
    expect(snippets.sync).toContain('"mode":"sync"');
    expect(snippets.sync).toContain("X-API-Key: apk_demo_secret");
    expect(snippets.stream).toContain("curl -N");
    expect(snippets.async).toContain("Idempotency-Key");
    expect(snippets.nodeVerify).toContain("process.env.WEBHOOK_SECRET");
    expect(snippets.nodeVerify).toContain("> 300");
    expect(snippets.nodeVerify).toContain("claimWebhook(id)");
    expect(snippets.nodeVerify).toContain("atomically insert");
    expect(snippets.nodeVerify).not.toContain("wasProcessed");
    expect(snippets.nodeVerify).not.toContain("markProcessed");
    expect(snippets.nodeVerify).not.toContain("apk_demo_secret");
    expect(snippets.goVerify).toContain('os.Getenv("WEBHOOK_SECRET")');
    expect(snippets.goVerify).toContain("hmac.Equal");
    expect(snippets.goVerify).toContain("> 300");
    expect(snippets.goVerify).toContain("claimWebhook(id)");
    expect(snippets.goVerify).toContain("unique constraint");
    expect(snippets.goVerify).not.toContain("seen func");
    expect(snippets.goVerify).not.toContain("mark func");
  });
});
