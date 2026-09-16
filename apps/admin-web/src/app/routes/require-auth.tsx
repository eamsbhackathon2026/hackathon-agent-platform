import { useEffect, useState } from "react";
import { Navigate, Outlet, useLocation } from "react-router";

import { useAuthSession } from "@/entities/session-user";
import { refreshSingleFlight } from "@/shared/api";
import { Skeleton } from "@/shared/ui";

export function RequireAuth() {
  const session = useAuthSession();
  const location = useLocation();
  const [ready, setReady] = useState(Boolean(session.accessToken));

  useEffect(() => {
    if (session.accessToken) return;
    let current = true;
    void refreshSingleFlight().finally(() => current && setReady(true));
    return () => { current = false; };
  }, [session.accessToken]);

  if (!ready && !session.accessToken) return <main className="grid min-h-screen place-items-center"><Skeleton className="h-72 w-full max-w-xl" /></main>;
  if (!session.user) {
    const returnTo = `${location.pathname}${location.search}`;
    return <Navigate to={`/login?returnTo=${encodeURIComponent(returnTo)}`} replace />;
  }
  if (session.user.must_change_password && location.pathname !== "/change-password") return <Navigate to="/change-password" replace />;
  return <Outlet />;
}
