import { isStructuredParam, type ToolParam } from "@/entities/tool";

export type ParameterRow = ToolParam & { rowId: string };

/**
 * Sets or clears the element type of a list input. The key is removed rather than set
 * to undefined, because the project compiles with exactOptionalPropertyTypes.
 */
export function withItemType(row: ParameterRow, value: string): ParameterRow {
  const { item_type: previous, ...rest } = row;
  void previous;
  return value ? { ...rest, item_type: value as NonNullable<ParameterRow["item_type"]> } : rest;
}

export function rowsToParams(rows: ParameterRow[]): ToolParam[] {
  const seen = new Set<string>();
  return rows.map((row) => {
    const { rowId, ...param } = row;
    void rowId;
    const normalized = param.name.trim().toLowerCase();
    if (!normalized) throw new Error("Each input needs a name");
    if (seen.has(normalized)) throw new Error("Input names must be unique");
    if (isStructuredParam(param.type) && param.in !== "body") throw new Error("Group and list inputs must be sent in the request body");
    seen.add(normalized);
    // Drop an inner declaration left behind by an earlier type: the API rejects fields on
    // anything but a group, and an element type on anything but a list.
    const { fields, item_type: itemType, ...rest } = param;
    const cleaned: ToolParam = { ...rest, name: param.name.trim(), description: param.description.trim() };
    if (param.type === "object" && fields?.length) cleaned.fields = fields.map((field) => ({ ...field, name: field.name.trim(), description: field.description.trim() }));
    if (param.type === "array" && itemType) cleaned.item_type = itemType;
    return cleaned;
  });
}
