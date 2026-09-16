import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router";
import { z } from "zod";

import { apiClient, getAuthSession, updateSessionUser, type components } from "@/shared/api";
import { problemToAction } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Button, Form, FormControl, FormField, FormItem, FormLabel, FormMessage, Input } from "@/shared/ui";

const passwordSchema = z.object({
  currentPassword: z.string().min(1, "Enter your current password."),
  newPassword: z.string().min(10, "The new password must contain at least 10 characters.").max(1024),
  confirmation: z.string(),
}).refine((values) => values.newPassword === values.confirmation, { path: ["confirmation"], message: "The passwords do not match." });

type PasswordValues = z.infer<typeof passwordSchema>;
type Problem = components["schemas"]["Problem"];

export function ChangePasswordForm() {
  const navigate = useNavigate();
  const [failure, setFailure] = useState<{ title: string; description: string } | null>(null);
  const form = useForm<PasswordValues>({ resolver: zodResolver(passwordSchema), defaultValues: { currentPassword: "", newPassword: "", confirmation: "" } });

  async function submit(values: PasswordValues) {
    setFailure(null);
    const { error, response } = await apiClient.POST("/v1/me/password", { body: { current_password: values.currentPassword, new_password: values.newPassword } });
    if (!response.ok) {
      setFailure(problemToAction((error as Problem | undefined)?.code));
      return;
    }
    const user = getAuthSession().user;
    if (user) updateSessionUser({ ...user, must_change_password: false });
    navigate("/agents", { replace: true });
  }

  return (
    <Form {...form}>
      <form className="space-y-5" onSubmit={form.handleSubmit(submit)} noValidate>
        {failure ? <Alert variant="destructive"><AlertTitle>{failure.title}</AlertTitle><AlertDescription>{failure.description}</AlertDescription></Alert> : null}
        <FormField control={form.control} name="currentPassword" render={({ field }) => <FormItem><FormLabel>Current password</FormLabel><FormControl><Input type="password" autoComplete="current-password" {...field} /></FormControl><FormMessage /></FormItem>} />
        <FormField control={form.control} name="newPassword" render={({ field }) => <FormItem><FormLabel>New password</FormLabel><FormControl><Input type="password" autoComplete="new-password" {...field} /></FormControl><FormMessage /></FormItem>} />
        <FormField control={form.control} name="confirmation" render={({ field }) => <FormItem><FormLabel>Confirm new password</FormLabel><FormControl><Input type="password" autoComplete="new-password" {...field} /></FormControl><FormMessage /></FormItem>} />
        <Button className="w-full" type="submit" disabled={form.formState.isSubmitting}>{form.formState.isSubmitting ? "Saving…" : "Change password"}</Button>
      </form>
    </Form>
  );
}
