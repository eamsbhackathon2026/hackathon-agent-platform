export type HeaderRow = { id: string; name: string; value: string };

export function headerRows(headers: Record<string, string>): HeaderRow[] {
  return Object.entries(headers).map(([name, value]) => ({ id: crypto.randomUUID(), name, value }));
}

export function headersRecord(rows: HeaderRow[]) {
  const result: Record<string, string> = {};
  const names = new Set<string>();
  for (const row of rows) {
    const name = row.name.trim();
    if (!name && !row.value) continue;
    if (!name || !row.value) throw new Error("Enter both a name and value for each header");
    const normalized = name.toLowerCase();
    if (names.has(normalized)) throw new Error("Header names must be unique");
    names.add(normalized);
    result[name] = row.value;
  }
  return result;
}

export function secretHeadersPatch(editing: boolean, replace: boolean, rows: HeaderRow[]) {
  if (editing && !replace) return {};
  const secretHeaders = headersRecord(rows);
  return Object.keys(secretHeaders).length || replace ? { secret_headers: secretHeaders } : {};
}

export function validateDistinctHeaderNames(publicHeaders: Record<string, string>, secretHeaders: Record<string, string> | undefined) {
  if (!secretHeaders) return;
  const publicNames = new Set(Object.keys(publicHeaders).map((name) => name.toLowerCase()));
  const duplicate = Object.keys(secretHeaders).find((name) => publicNames.has(name.toLowerCase()));
  if (duplicate) throw new Error(`Public and secret headers cannot share a name: ${duplicate}`);
}
