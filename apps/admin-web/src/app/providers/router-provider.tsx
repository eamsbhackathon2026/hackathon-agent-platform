import { RouterProvider } from "react-router";

import { router } from "../routes/route-table";

export function AppRouterProvider() {
  return <RouterProvider router={router} />;
}
