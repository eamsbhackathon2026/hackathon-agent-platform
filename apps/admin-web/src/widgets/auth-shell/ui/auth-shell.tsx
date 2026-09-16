import { ShieldCheck, Waypoints } from "lucide-react";
import type { ReactNode } from "react";

type AuthShellProps = {
  eyebrow: string;
  title: string;
  description: string;
  children: ReactNode;
  footer?: ReactNode;
};

export function AuthShell({ eyebrow, title, description, children, footer }: AuthShellProps) {
  return (
    <main className="grid min-h-dvh place-items-center px-4 py-8 sm:px-6">
      <section className="grid w-full max-w-5xl overflow-hidden rounded-[24px] border bg-card shadow-[var(--shadow-float)] lg:min-h-[620px] lg:grid-cols-[0.92fr_1.08fr]">
        <aside className="auth-shell-art hidden flex-col justify-between p-10 text-sidebar-foreground lg:flex">
          <div className="flex items-center gap-3">
            <span className="grid size-12 place-items-center rounded-[15px] bg-brand text-white shadow-lg"><Waypoints className="size-7" strokeWidth={1.8} /></span>
            <div><p className="font-semibold tracking-[-0.02em]">Agent Platform</p><p className="mt-0.5 text-xs text-sidebar-muted">Workspace control</p></div>
          </div>
          <div className="max-w-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-sidebar-accent">Built for dependable work</p>
            <h2 className="mt-4 text-3xl font-semibold leading-tight tracking-[-0.035em]">Your assistants, connections, and activity in one trusted workspace.</h2>
            <div className="mt-7 flex items-center gap-3 border-t border-white/15 pt-6 text-sm text-sidebar-muted"><ShieldCheck className="size-5 shrink-0 text-sidebar-accent" /><span>Secure access with clear roles and auditable activity.</span></div>
          </div>
        </aside>
        <div className="flex flex-col justify-center p-5 sm:p-8 lg:p-12">
          <div className="mb-8 flex items-center gap-3 lg:hidden">
            <span className="grid size-11 place-items-center rounded-[14px] bg-brand text-white"><Waypoints className="size-6" strokeWidth={1.8} /></span>
            <div><p className="font-semibold tracking-[-0.02em]">Agent Platform</p><p className="text-xs text-muted-foreground">Workspace control</p></div>
          </div>
          <div className="mb-7">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-primary">{eyebrow}</p>
            <h1 className="mt-3 text-3xl font-semibold leading-tight tracking-[-0.035em] text-foreground">{title}</h1>
            <p className="mt-3 max-w-md text-sm leading-6 text-muted-foreground">{description}</p>
          </div>
          {children}
          {footer ? <div className="mt-7 border-t pt-6 text-center text-sm text-muted-foreground">{footer}</div> : null}
        </div>
      </section>
    </main>
  );
}
