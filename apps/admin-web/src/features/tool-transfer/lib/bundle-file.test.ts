import { describe, expect, it } from "vitest";
import { bundleFileName, maxBundleBytes, readBundleFile, transferErrorMessage } from "./bundle-file";

describe("bundle file helpers", () => {
  it("accepts only exported tools files within the size limit", async () => {
    await expect(readBundleFile(new File(['{"format":"agent-platform.tools","version":1,"tools":[],"connections":[]}'], "t.json"))).resolves.toMatchObject({ version: 1 });
    await expect(readBundleFile(new File(['{"format":"other"}'], "t.json"))).rejects.toThrow("not created with Export tools");
    await expect(readBundleFile(new File(["x".repeat(maxBundleBytes + 1)], "t.json"))).rejects.toThrow("larger than 2 MiB");
    await expect(readBundleFile(new File(['{"format":"agent-platform.tools","version":2,"tools":[],"connections":[]}'], "t.json"))).rejects.toThrow("different version");
    await expect(readBundleFile(new File(['{"format":"agent-platform.tools","version":1}'], "t.json"))).rejects.toThrow("incomplete");
    await expect(readBundleFile(new File([JSON.stringify({ format: "agent-platform.tools", version: 1, connections: [], tools: new Array(201).fill({}) })], "t.json"))).rejects.toThrow("at most 200 tools");
  });

  it("names the download after the local date", () => {
    expect(bundleFileName(new Date(2026, 8, 7))).toBe("tools-20260907.json");
  });

  it("prefers server detail and field reasons over the fallback", () => {
    expect(transferErrorMessage({ detail: "Nothing was saved.", fields: [{ message: "Bad address." }] }, "fallback")).toBe("Nothing was saved. Bad address.");
    expect(transferErrorMessage(undefined, "fallback")).toBe("fallback");
  });
});
