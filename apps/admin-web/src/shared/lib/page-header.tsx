import type { ReactNode } from "react";

import { PageHeaderContext, type SetPageHeader } from "./page-header-context";

export function PageHeaderProvider({ onChange, children }: { onChange: SetPageHeader; children: ReactNode }) {
  return <PageHeaderContext.Provider value={onChange}>{children}</PageHeaderContext.Provider>;
}
