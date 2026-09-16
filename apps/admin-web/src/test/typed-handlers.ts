import { HttpResponse, type JsonBodyType } from "msw";

import type { paths } from "@/shared/api";

export type ApiPath = keyof paths;

export function apiUrl<TPath extends ApiPath>(path: TPath) {
  return `*${path}` as const;
}

export function jsonResponse<T extends JsonBodyType>(body: T, status = 200) {
  return HttpResponse.json(body, { status });
}
