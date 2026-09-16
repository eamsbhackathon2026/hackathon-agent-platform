import { Navigate, createBrowserRouter } from "react-router";

import { AppShell } from "@/widgets/app-shell";
import { Skeleton } from "@/shared/ui";

import { RequireAuth } from "./require-auth";
import { RequireWorkspaceAdmin } from "./require-workspace-admin";

function RouteLoading() {
  return <main className="grid min-h-screen place-items-center"><Skeleton className="h-72 w-full max-w-xl" /></main>;
}

function lazyPage(loader: () => Promise<{ Component: React.ComponentType }>) {
  return { lazy: loader, HydrateFallback: RouteLoading };
}

export const router = createBrowserRouter([
  { path: "/login", ...lazyPage(() => import("@/pages/login")) },
  { path: "/setup", ...lazyPage(() => import("@/pages/first-run-setup")) },
  {
    Component: RequireAuth,
    children: [
      { path: "/change-password", ...lazyPage(() => import("@/pages/change-password")) },
      {
        Component: AppShell,
        children: [
          { index: true, element: <Navigate to="/agents" replace /> },
          { path: "agents", ...lazyPage(() => import("@/pages/agents")) },
          { path: "agents/new", ...lazyPage(() => import("@/pages/agent-create")) },
          { path: "agents/:agentId", ...lazyPage(() => import("@/pages/agent-detail")) },
          { path: "playground", ...lazyPage(() => import("@/pages/playground")) },
          { path: "conversations", ...lazyPage(() => import("@/pages/conversations")) },
          { path: "conversations/:sessionId", ...lazyPage(() => import("@/pages/conversation-detail")) },
          { path: "activity", ...lazyPage(() => import("@/pages/activity")) },
          { path: "activity/:runId", ...lazyPage(() => import("@/pages/activity-detail")) },
          { path: "connections", ...lazyPage(() => import("@/pages/connections")) },
          { path: "tools", ...lazyPage(() => import("@/pages/tools")) },
          { path: "tools/new", ...lazyPage(() => import("@/pages/tool-edit")) },
          { path: "tools/:toolId/edit", ...lazyPage(() => import("@/pages/tool-edit")) },
          { path: "skills", ...lazyPage(() => import("@/pages/skills")) },
          {
            Component: RequireWorkspaceAdmin,
            children: [
              { path: "integrations", ...lazyPage(() => import("@/pages/integrations")) },
              { path: "members", ...lazyPage(() => import("@/pages/members")) },
            ],
          },
        ],
      },
    ],
  },
  { path: "*", ...lazyPage(() => import("@/pages/not-found")) },
]);
