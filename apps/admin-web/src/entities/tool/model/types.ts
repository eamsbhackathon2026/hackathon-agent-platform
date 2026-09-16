import type { components } from "@/shared/api";

export type HttpTool = components["schemas"]["HttpTool"];
export type ToolParam = components["schemas"]["ToolParam"];
export type ToolParamField = components["schemas"]["ToolParamField"];

/**
 * Groups and lists carry nested JSON. The service only accepts them in a request
 * body, because path and query values must serialize to a single string.
 */
export const isStructuredParam = (type: ToolParam["type"]) => type === "object" || type === "array";
