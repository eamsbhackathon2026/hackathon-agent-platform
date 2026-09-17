import { Navigate } from "react-router";

import { canManageWorkspace, useAuthSession } from "@/entities/session-user";

/**
 * Owners and administrators land on the workspace overview; members cannot see
 * workspace-wide reporting, so they land on the assistants they work with.
 */
export function HomeRedirect() {
  const { user } = useAuthSession();
  const target = user && canManageWorkspace(user.role) ? "/overview" : "/agents";
  return <Navigate to={target} replace />;
}
