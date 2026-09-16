import { Link } from "react-router";

import { Button } from "@/shared/ui";

export function NotFoundPage() {
  return <main className="grid min-h-screen place-items-center px-4 text-center"><div><p className="text-sm font-semibold text-primary">404</p><h1 className="mt-2 text-3xl font-semibold">Page not found</h1><p className="mt-3 text-muted-foreground">This address may have changed or no longer exists.</p><Button asChild className="mt-6"><Link to="/agents">Back to AI Assistants</Link></Button></div></main>;
}
