import { LogOut } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router";

import { apiClient, clearAuthSession } from "@/shared/api";
import { Button, DropdownMenuItem } from "@/shared/ui";

export function LogoutButton({ appearance = "button" }: { appearance?: "button" | "menu-item" }) {
  const navigate = useNavigate();
  const [pending, setPending] = useState(false);

  async function logout() {
    setPending(true);
    try {
      await apiClient.POST("/v1/auth/logout");
    } finally {
      navigate("/login", { replace: true });
      clearAuthSession();
    }
  }

  if (appearance === "menu-item") return <DropdownMenuItem className="text-destructive focus:text-destructive" disabled={pending} onSelect={() => void logout()}><LogOut />{pending ? "Signing out…" : "Sign out"}</DropdownMenuItem>;
  return <Button variant="ghost" className="w-full justify-start" onClick={logout} disabled={pending}><LogOut />{pending ? "Signing out…" : "Sign out"}</Button>;
}
