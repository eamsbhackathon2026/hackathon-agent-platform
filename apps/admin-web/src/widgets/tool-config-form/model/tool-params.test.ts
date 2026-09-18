import { describe, expect, it } from "vitest";
import { rowsToParams, withItemType } from "./tool-params";

describe("rowsToParams", () => {
  it("maps editable rows to the API payload", () => {
    expect(rowsToParams([{ rowId: "1", name: "order_id", type: "string", description: "Order ID", required: true, in: "path", show_in_progress: false }])).toEqual([{ name: "order_id", type: "string", description: "Order ID", required: true, in: "path", show_in_progress: false }]);
  });
  it("keeps a group input that is sent in the request body", () => {
    expect(rowsToParams([{ rowId: "1", name: "session_flags", type: "object", description: "Session signals", required: false, in: "body", show_in_progress: false }])).toEqual([
      { name: "session_flags", type: "object", description: "Session signals", required: false, in: "body", show_in_progress: false },
    ]);
  });
  it("rejects a group or list input outside the request body", () => {
    for (const location of ["path", "query"] as const) {
      expect(() => rowsToParams([{ rowId: "1", name: "session_flags", type: "object", description: "", required: true, in: location, show_in_progress: false }])).toThrow("request body");
      expect(() => rowsToParams([{ rowId: "1", name: "recent_events", type: "array", description: "", required: true, in: location, show_in_progress: false }])).toThrow("request body");
    }
  });
  it("keeps a declared group shape and list element type", () => {
    expect(rowsToParams([
      { rowId: "1", name: "session_flags", type: "object", description: "Session signals", required: false, in: "body", show_in_progress: false, fields: [{ name: " screen_sharing ", type: "boolean", description: " Sharing ", required: true }] },
      { rowId: "2", name: "recent_events", type: "array", description: "Codes", required: false, in: "body", show_in_progress: false, item_type: "string" },
    ])).toEqual([
      { name: "session_flags", type: "object", description: "Session signals", required: false, in: "body", show_in_progress: false, fields: [{ name: "screen_sharing", type: "boolean", description: "Sharing", required: true }] },
      { name: "recent_events", type: "array", description: "Codes", required: false, in: "body", show_in_progress: false, item_type: "string" },
    ]);
  });
  it("drops an inner declaration left behind by an earlier type", () => {
    expect(rowsToParams([
      { rowId: "1", name: "note", type: "string", description: "", required: false, in: "query", show_in_progress: false, fields: [{ name: "stale", type: "boolean", description: "", required: false }], item_type: "string" },
    ])).toEqual([{ name: "note", type: "string", description: "", required: false, in: "query", show_in_progress: false }]);
  });
  it("rejects duplicate names regardless of casing", () => {
    expect(() => rowsToParams([
      { rowId: "1", name: "Code", type: "string", description: "", required: false, in: "query", show_in_progress: false },
      { rowId: "2", name: "code", type: "string", description: "", required: false, in: "query", show_in_progress: false },
    ])).toThrow("must be unique");
  });
  it("keeps show_in_progress enabled when the operator opted a value in", () => {
    expect(rowsToParams([{ rowId: "1", name: "month", type: "string", description: "Month", required: true, in: "query", show_in_progress: true }])).toEqual([
      { name: "month", type: "string", description: "Month", required: true, in: "query", show_in_progress: true },
    ]);
  });
});

describe("withItemType", () => {
  it("removes the key instead of storing undefined", () => {
    const row = { rowId: "1", name: "events", type: "array", description: "", required: false, in: "body", show_in_progress: false, item_type: "string" } as const;
    expect(Object.hasOwn(withItemType({ ...row }, ""), "item_type")).toBe(false);
    expect(withItemType({ ...row }, "integer").item_type).toBe("integer");
  });
});
