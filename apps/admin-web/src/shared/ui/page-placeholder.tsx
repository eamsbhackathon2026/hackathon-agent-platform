import type { LucideIcon } from "lucide-react";

import { Card, CardContent } from "./card";

type PagePlaceholderProps = {
  title: string;
  description: string;
  icon: LucideIcon;
};

export function PagePlaceholder({ title, description, icon: Icon }: PagePlaceholderProps) {
  return <div className="mx-auto max-w-5xl"><div className="mb-8"><h1 className="text-2xl font-semibold tracking-tight">{title}</h1><p className="mt-2 text-sm text-muted-foreground">{description}</p></div><Card className="border-dashed"><CardContent className="flex min-h-72 flex-col items-center justify-center text-center"><span className="rounded-2xl bg-brand-soft p-4 text-primary"><Icon className="size-7" /></span><h2 className="mt-5 text-lg font-medium">Coming soon</h2><p className="mt-2 max-w-md text-sm text-muted-foreground">The platform is ready. This page will be connected to live data in the next step.</p></CardContent></Card></div>;
}
