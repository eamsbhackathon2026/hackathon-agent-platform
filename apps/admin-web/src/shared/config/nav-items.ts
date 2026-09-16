import { Activity, Bot, History, KeyRound, LibraryBig, MessageSquareText, PlugZap, Users, Wrench, type LucideIcon } from "lucide-react";

import type { components } from "@/shared/api";

type Role = components["schemas"]["Role"];

export type NavItem = {
  label: string;
  to: string;
  icon: LucideIcon;
  roles?: Role[];
  showInNavigation?: boolean;
};

export const navItems: NavItem[] = [
  { label: "AI Assistants", to: "/agents", icon: Bot },
  { label: "Playground", to: "/playground", icon: MessageSquareText },
  { label: "Conversation History", to: "/conversations", icon: History, showInNavigation: false },
  { label: "Activity", to: "/activity", icon: Activity },
  { label: "Model Connections", to: "/connections", icon: PlugZap },
  { label: "Tools", to: "/tools", icon: Wrench },
  { label: "Skill Hub", to: "/skills", icon: LibraryBig },
  { label: "API Access", to: "/integrations", icon: KeyRound, roles: ["owner", "admin"] },
  { label: "Members", to: "/members", icon: Users, roles: ["owner", "admin"] },
];
