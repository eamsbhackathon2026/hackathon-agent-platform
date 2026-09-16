import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useLocation, useNavigate } from "react-router";
import { z } from "zod";

import { apiClient, setAuthSession, type components } from "@/shared/api";
import { problemToAction } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Button, Form, FormControl, FormField, FormItem, FormLabel, FormMessage, Input } from "@/shared/ui";

const loginSchema = z.object({
  email: z.string().trim().email("Enter a valid email address."),
  password: z.string().min(1, "Enter your password."),
});

type LoginValues = z.infer<typeof loginSchema>;
type Problem = components["schemas"]["Problem"];

export function LoginForm() {
  const navigate = useNavigate();
  const location = useLocation();
  const [failure, setFailure] = useState<{ title: string; description: string } | null>(null);
  const form = useForm<LoginValues>({ resolver: zodResolver(loginSchema), defaultValues: { email: "", password: "" } });

  async function submit(values: LoginValues) {
    setFailure(null);
    const { data, error } = await apiClient.POST("/v1/auth/login", { body: values });
    if (!data) {
      const action = problemToAction((error as Problem | undefined)?.code);
      setFailure(action);
      return;
    }
    setAuthSession(data.access_token, data.me.user);
    const returnTo = new URLSearchParams(location.search).get("returnTo");
    navigate(data.me.user.must_change_password ? "/change-password" : returnTo?.startsWith("/") ? returnTo : "/agents", { replace: true });
  }

  return (
    <Form {...form}>
      <form className="space-y-5" onSubmit={form.handleSubmit(submit)} noValidate>
        {failure ? <Alert variant="destructive"><AlertTitle>{failure.title}</AlertTitle><AlertDescription>{failure.description}</AlertDescription></Alert> : null}
        <FormField control={form.control} name="email" render={({ field }) => <FormItem><FormLabel>Email</FormLabel><FormControl><Input autoComplete="email" type="email" placeholder="you@company.com" {...field} /></FormControl><FormMessage /></FormItem>} />
        <FormField control={form.control} name="password" render={({ field }) => <FormItem><FormLabel>Password</FormLabel><FormControl><Input autoComplete="current-password" type="password" {...field} /></FormControl><FormMessage /></FormItem>} />
        <Button className="w-full" type="submit" disabled={form.formState.isSubmitting}>{form.formState.isSubmitting ? "Signing in…" : "Sign in"}</Button>
      </form>
    </Form>
  );
}
