import { Link, Outlet } from "react-router";

import { canManageWorkspace, useAuthSession } from "@/entities/session-user";
import { Alert, AlertDescription, AlertTitle, Button } from "@/shared/ui";

export function RequireWorkspaceAdmin() {
  const { user } = useAuthSession();
  if (user && canManageWorkspace(user.role)) return <Outlet />;
  return <Alert><AlertTitle>You do not have access to this page</AlertTitle><AlertDescription className="mt-2">Only owners and administrators can change workspace settings.</AlertDescription><Button asChild className="mt-4"><Link to="/agents">Back to AI Assistants</Link></Button></Alert>;
}
