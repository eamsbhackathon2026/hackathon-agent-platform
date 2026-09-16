type Page<T> = { items: T[]; next_cursor: string | null };
type PageQuery = { limit: number; cursor?: string };

/**
 * Walks a cursor-paginated list endpoint to the end and returns every item.
 *
 * Screens that render the whole collection at once (checkbox pickers, lookups by
 * id) must not rely on the server's default page size, which is 20; a list that
 * silently stops at the first page looks complete while hiding most of the data.
 */
export async function fetchAllPages<T>(fetchPage: (query: PageQuery) => Promise<Page<T>>, label: string): Promise<T[]> {
  const items: T[] = [];
  let cursor: string | undefined;
  do {
    const page = await fetchPage({ limit: 100, ...(cursor ? { cursor } : {}) });
    items.push(...page.items);
    const next = page.next_cursor ?? undefined;
    if (next && next === cursor) throw new Error(`${label} pagination did not advance`);
    cursor = next;
  } while (cursor);
  return items;
}
