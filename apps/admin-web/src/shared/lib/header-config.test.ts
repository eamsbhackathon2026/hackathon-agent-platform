import { describe, expect, it } from "vitest";
import { headerRows, headersRecord, secretHeadersPatch, validateDistinctHeaderNames } from "./header-config";

describe("header configuration", () => {
  it("preloads and preserves all public values", () => {
    expect(headersRecord(headerRows({ Accept: "application/json", "X-App": "admin" }))).toEqual({ Accept: "application/json", "X-App": "admin" });
  });
  it("omits saved secrets until replacement is explicitly selected", () => {
    expect(secretHeadersPatch(true, false, [])).toEqual({});
  });
  it("allows clearing or renaming the complete saved secret set", () => {
    expect(secretHeadersPatch(true, true, [])).toEqual({ secret_headers: {} });
    expect(secretHeadersPatch(true, true, [{ id: "1", name: "X-New-Token", value: "secret" }])).toEqual({ secret_headers: { "X-New-Token": "secret" } });
  });
  it("rejects public and secret names that only differ by case", () => {
    expect(() => validateDistinctHeaderNames({ Authorization: "public" }, { authorization: "secret" })).toThrow("cannot share a name");
  });
});
