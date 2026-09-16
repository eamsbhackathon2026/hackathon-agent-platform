import { Link, Navigate } from "react-router";

import { RegisterForm, useAuthConfig } from "@/features/auth-register";
import { Alert, AlertDescription, AlertTitle, Button, Skeleton } from "@/shared/ui";
import { AuthShell } from "@/widgets/auth-shell";

export function FirstRunSetupPage() {
  const config = useAuthConfig();
  if (config.isLoading) return <main className="grid min-h-screen place-items-center"><Skeleton className="h-96 w-full max-w-md" /></main>;
  if (config.isError) return <main className="grid min-h-screen place-items-center px-4"><Alert className="max-w-md"><AlertTitle>Unable to check setup status</AlertTitle><AlertDescription className="mt-3">Check the server connection and try again.</AlertDescription><Button className="mt-4" onClick={() => void config.refetch()}>Try again</Button></Alert></main>;
  if (!config.data?.signup_allowed) return <Navigate to="/login" replace />;
  return (
    <AuthShell eyebrow="One-time setup" title="Create the owner account" description="Enter your details. You can invite more members later." footer={<Link className="font-semibold text-primary underline-offset-4 hover:underline" to="/login">Back to sign in</Link>}>
      <RegisterForm />
    </AuthShell>
  );
}
