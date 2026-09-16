import { ChangePasswordForm } from "@/features/auth-change-password";
import { AuthShell } from "@/widgets/auth-shell";

export function ChangePasswordPage() {
  return <AuthShell eyebrow="Account security" title="Change password" description="Create a personal password before you continue using the platform."><ChangePasswordForm /></AuthShell>;
}
