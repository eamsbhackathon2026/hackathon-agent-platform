import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router";
import { z } from "zod";

import { apiClient, setAuthSession, type components } from "@/shared/api";
import { problemToAction } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Button, Form, FormControl, FormField, FormItem, FormLabel, FormMessage, Input } from "@/shared/ui";

const registerSchema = z.object({
  name: z.string().trim().min(1, "Enter your display name.").max(200),
  email: z.string().trim().email("Enter a valid email address."),
  password: z.string().min(10, "The password must contain at least 10 characters.").max(1024),
});

type RegisterValues = z.infer<typeof registerSchema>;
type Problem = components["schemas"]["Problem"];

export function RegisterForm() {
  const navigate = useNavigate();
  const [failure, setFailure] = useState<{ title: string; description: string } | null>(null);
  const form = useForm<RegisterValues>({ resolver: zodResolver(registerSchema), defaultValues: { name: "", email: "", password: "" } });

  async function submit(values: RegisterValues) {
    setFailure(null);
    const { data, error, response } = await apiClient.POST("/v1/auth/register", { body: values });
    if (!data) {
      if (response.status === 409) {
        setFailure({ title: "Unable to complete initial setup", description: "Another person has already completed setup. Continue to sign in." });
      } else {
        setFailure(problemToAction((error as Problem | undefined)?.code));
      }
      return;
    }
    setAuthSession(data.access_token, data.me.user);
    navigate("/agents", { replace: true });
  }

  return (
    <Form {...form}>
      <form className="space-y-5" onSubmit={form.handleSubmit(submit)} noValidate>
        {failure ? <Alert variant="destructive"><AlertTitle>{failure.title}</AlertTitle><AlertDescription>{failure.description}</AlertDescription></Alert> : null}
        <FormField control={form.control} name="name" render={({ field }) => <FormItem><FormLabel>Your name</FormLabel><FormControl><Input autoComplete="name" {...field} /></FormControl><FormMessage /></FormItem>} />
        <FormField control={form.control} name="email" render={({ field }) => <FormItem><FormLabel>Email</FormLabel><FormControl><Input autoComplete="email" type="email" {...field} /></FormControl><FormMessage /></FormItem>} />
        <FormField control={form.control} name="password" render={({ field }) => <FormItem><FormLabel>Password</FormLabel><FormControl><Input autoComplete="new-password" type="password" {...field} /></FormControl><FormMessage /></FormItem>} />
        <Button className="w-full" type="submit" disabled={form.formState.isSubmitting}>{form.formState.isSubmitting ? "Setting up…" : "Complete setup"}</Button>
      </form>
    </Form>
  );
}
