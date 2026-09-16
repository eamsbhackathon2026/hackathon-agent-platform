import type { ApiConnection } from "@/entities/api-connection";

type Props = { connection: ApiConnection; path: string };

/**
 * Shows what a chosen API connection contributes to this tool: the address the
 * request will actually hit and the headers it will carry. Secret values are
 * never available to the browser, so only their names appear.
 */
export function ConnectionSummary({ connection, path }: Props) {
  const publicHeaders = Object.entries(connection.public_headers).sort(([left], [right]) => left.localeCompare(right));
  const secretNames = [...connection.secret_header_names].sort((left, right) => left.localeCompare(right));
  const address = `${connection.base_url.replace(/\/+$/u, "")}${path.trim() || "/…"}`;
  return <dl className="grid gap-2 rounded-md border bg-muted/40 p-3 text-sm md:grid-cols-[auto_1fr] md:gap-x-4" role="group" aria-label="Connection details">
    <dt className="text-muted-foreground">Full address</dt>
    <dd><code className="break-all">{address}</code></dd>
    <dt className="text-muted-foreground">Shared headers</dt>
    <dd>
      {publicHeaders.length === 0 && secretNames.length === 0 ? <span>None. Requests carry only the headers defined below.</span> : <ul className="grid gap-1">
        {publicHeaders.map(([name, value]) => <li key={name}><code>{name}</code>: {value}</li>)}
        {secretNames.map((name) => <li key={name}><code>{name}</code>: <span className="text-muted-foreground">secret value, never shown</span></li>)}
      </ul>}
    </dd>
    <dt className="text-muted-foreground">Manage</dt>
    <dd className="text-muted-foreground">Change the base URL or shared headers under the API connections tab; every tool using <span className="font-medium text-foreground">{connection.display_name}</span> follows.</dd>
  </dl>;
}
