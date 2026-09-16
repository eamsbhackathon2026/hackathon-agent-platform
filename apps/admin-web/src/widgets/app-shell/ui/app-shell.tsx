import { Ellipsis, KeyRound, Menu, Waypoints } from "lucide-react";
import { Link, NavLink, Outlet, useLocation } from "react-router";

import { canManageWorkspace, roleLabel, useAuthSession } from "@/entities/session-user";
import { LogoutButton } from "@/features/auth-logout";
import { cn } from "@/shared/lib";
import { navItems } from "@/shared/config";
import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Sheet,
  SheetContent,
  SheetTitle,
  SheetTrigger,
} from "@/shared/ui";

function Navigation({ onNavigate }: { onNavigate?: () => void }) {
  const { user } = useAuthSession();
  if (!user) return null;
  return <nav className="space-y-1.5" aria-label="Main navigation">
    {navItems.filter((item) => item.showInNavigation !== false && (!item.roles || canManageWorkspace(user.role))).map((item) => <NavLink
      key={item.to}
      to={item.to}
      onClick={onNavigate}
      className={({ isActive }) => cn(
        "flex min-h-11 items-center gap-3 rounded-xl border-l-2 px-3 py-2.5 text-sm font-medium transition-[color,background-color,border-color] duration-200",
        isActive ? "border-brand bg-white/10 text-sidebar-foreground" : "border-transparent text-sidebar-muted hover:bg-white/[0.07] hover:text-sidebar-foreground",
      )}
    >
      <item.icon className="size-[18px]" strokeWidth={1.8} />
      {item.label}
    </NavLink>)}
  </nav>;
}

function Brand() {
  return <div className="flex items-center gap-3">
    <span className="grid size-11 place-items-center rounded-[14px] bg-brand text-white shadow-[0_10px_24px_rgb(0_0_0/0.18)]"><Waypoints className="size-6" strokeWidth={1.9} /></span>
    <div><p className="font-semibold tracking-[-0.02em] text-sidebar-foreground">Agent Platform</p><p className="mt-0.5 text-xs text-sidebar-muted">Workspace control</p></div>
  </div>;
}

function AccountMenu({ mobile = false }: { mobile?: boolean }) {
  const { user } = useAuthSession();
  if (!user) return null;
  const initial = user.name.trim().charAt(0).toUpperCase() || user.email.charAt(0).toUpperCase();

  return <DropdownMenu>
    <div aria-label="Current user" className="flex items-center gap-2 rounded-xl p-2 transition-colors hover:bg-white/[0.05]" role="group">
      <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-lg bg-brand text-sm font-semibold text-white">{initial}</span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-sidebar-foreground">{user.name}</p>
        <p className="mt-0.5 truncate text-xs text-sidebar-muted">{roleLabel(user.role)}</p>
      </div>
      <DropdownMenuTrigger asChild>
        <Button aria-label={`Open account menu for ${user.name}`} className="size-9 shrink-0 rounded-lg p-0 text-sidebar-muted hover:bg-white/10 hover:text-sidebar-foreground" size="icon" variant="ghost">
          <Ellipsis aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
    </div>
    <DropdownMenuContent align="end" className="w-60" side={mobile ? "top" : "right"} sideOffset={8}>
      <DropdownMenuLabel>
        <p className="truncate">{user.name}</p>
        <p className="mt-0.5 truncate font-normal text-muted-foreground">{user.email}</p>
      </DropdownMenuLabel>
      <DropdownMenuSeparator />
      <DropdownMenuItem asChild>
        <Link to="/change-password"><KeyRound />Change password</Link>
      </DropdownMenuItem>
      <LogoutButton appearance="menu-item" />
    </DropdownMenuContent>
  </DropdownMenu>;
}

export function AppShell() {
  const location = useLocation();
  const title = navItems.find((item) => location.pathname.startsWith(item.to))?.label ?? "Agent Platform";

  return (
    <div className="min-h-dvh md:grid md:grid-cols-[272px_1fr]">
      <a href="#main-content" className="fixed left-4 top-4 z-[100] -translate-y-24 rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground shadow-lg transition-transform focus:translate-y-0">Skip to main content</a>
      <aside className="hidden border-r border-sidebar-border bg-sidebar px-4 py-6 text-sidebar-foreground md:sticky md:top-0 md:flex md:h-dvh md:flex-col md:overflow-y-auto">
        <div className="px-2"><Brand /></div>
        <div className="mt-9 flex-1"><Navigation /></div>
        <div className="border-t border-sidebar-border pt-3"><AccountMenu /></div>
      </aside>
      <div className="min-w-0">
        <header className="sticky top-0 z-20 flex min-h-18 items-center border-b-2 border-brand/40 bg-white/95 px-4 py-3 shadow-[0_5px_18px_rgb(23_35_59/0.04)] backdrop-blur-md md:px-8">
          <div className="flex items-center gap-3">
            <Sheet>
              <SheetTrigger asChild><Button className="md:hidden" size="icon" variant="ghost" aria-label="Open navigation"><Menu /></Button></SheetTrigger>
              <SheetContent side="left" className="flex w-72 flex-col border-sidebar-border bg-sidebar p-5 text-sidebar-foreground">
                <SheetTitle className="sr-only">Navigation</SheetTitle>
                <Brand />
                <div className="mt-8 flex-1"><Navigation /></div>
                <div className="border-t border-sidebar-border pt-3"><AccountMenu mobile /></div>
              </SheetContent>
            </Sheet>
            <div><p className="hidden text-[11px] font-semibold uppercase tracking-[0.16em] text-primary sm:block">Workspace</p><h1 className="text-lg font-semibold tracking-[-0.02em] text-foreground">{title}</h1></div>
          </div>
        </header>
        <div id="main-content" className="app-content p-4 pb-10 md:p-8 md:pb-12" role="main" tabIndex={-1}><Outlet /></div>
      </div>
    </div>
  );
}
