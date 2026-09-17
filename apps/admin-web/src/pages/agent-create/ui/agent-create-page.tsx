import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { toast } from "sonner";
import { createAgent } from "@/features/agent-upsert";
import { AgentForm, type AgentFormValues } from "@/widgets/agent-form";
import { useAuthSession } from "@/shared/api";
import { usePageHeader } from "@/shared/lib";
import { Button, Card, CardContent } from "@/shared/ui";

export function AgentCreatePage() {
  const navigate = useNavigate(); const [params] = useSearchParams(); const [saving, setSaving] = useState(false); const session = useAuthSession();
  const canCreate = session.user?.role !== "member";
  // A member only sees the "ask an administrator" card, so the header keeps the section title.
  usePageHeader(canCreate ? { title: "Create assistant", description: "Describe the work in plain language. You can change these settings later." } : null);
  const save = async (values: AgentFormValues) => { setSaving(true); try { const agent = await createAgent(values); toast.success("Assistant created", { action: { label: "Open playground", onClick: () => navigate(`/playground?agent=${agent.id}`) } }); navigate(`/agents/${agent.id}`); } catch { toast.error("Unable to save assistant", { description: "Review the information and try again." }); } finally { setSaving(false); } };
  if (!canCreate) return <Card><CardContent className="space-y-3 pt-6"><p>Only administrators can create assistants. Contact an administrator for help.</p><Button asChild variant="outline"><Link to="/agents">Back to list</Link></Button></CardContent></Card>;
  return <main className="space-y-6"><AgentForm preferredProviderId={params.get("provider") ?? undefined} saving={saving} onSubmit={save} /></main>;
}
