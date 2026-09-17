import { Activity, Bot, History, KeyRound, LayoutDashboard, LibraryBig, MessageSquareText, PlugZap, Users, Wrench, type LucideIcon } from "lucide-react";

import type { components } from "@/shared/api";

type Role = components["schemas"]["Role"];

export type NavItem = {
  label: string;
  /** Shown under the title in the app header; a detail page can replace it with `usePageHeader`. */
  description: string;
  to: string;
  icon: LucideIcon;
  roles?: Role[];
  showInNavigation?: boolean;
};

export const navItems: NavItem[] = [
  { label: "Overview", description: "See how much your assistants are used and what is failing, then open any request for detail.", to: "/overview", icon: LayoutDashboard, roles: ["owner", "admin"] },
  { label: "AI Assistants", description: "Create an assistant for each workflow and test it when it is ready.", to: "/agents", icon: Bot },
  { label: "Playground", description: "Send a request and follow the response in real time.", to: "/playground", icon: MessageSquareText },
  { label: "Conversation History", description: "Review conversations from the admin portal and external systems.", to: "/conversations", icon: History, showInNavigation: false },
  { label: "Activity", description: "Open any request to follow the agent loop and understand each observable decision.", to: "/activity", icon: Activity },
  { label: "Model Connections", description: "Connect the AI services that power your assistants.", to: "/connections", icon: PlugZap },
  { label: "Tools", description: "Let assistants retrieve data or perform work in external systems.", to: "/tools", icon: Wrench },
  { label: "Skill Hub", description: "Keep reusable instructions in one place, then choose the skills each assistant should follow.", to: "/skills", icon: LibraryBig },
  { label: "API Access", description: "Create keys so your applications can call assistants and retrieve results.", to: "/integrations", icon: KeyRound, roles: ["owner", "admin"] },
  { label: "Members", description: "Manage who can view and update assistants.", to: "/members", icon: Users, roles: ["owner", "admin"] },
];
