import { Link } from "react-router";

import { LoginForm } from "@/features/auth-login";
import { useAuthConfig } from "@/features/auth-register";
import { AuthShell } from "@/widgets/auth-shell";

export function LoginPage() {
  const config = useAuthConfig();
  return (
    <AuthShell eyebrow="Secure access" title="Welcome back" description="Sign in to manage and use your AI assistants." footer={config.data?.signup_allowed ? <>First time here? <Link className="font-semibold text-primary underline-offset-4 hover:underline" to="/setup">Set up now</Link></> : undefined}>
      <LoginForm />
    </AuthShell>
  );
}
