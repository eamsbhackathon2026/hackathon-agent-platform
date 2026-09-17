import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Users } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import {
  canAssignRole,
  canManageMembers,
  canModifyMember,
  memberQueries,
  MemberStatusBadge,
  type Member,
  type MemberCreated,
} from "@/entities/member";
import { addMember } from "@/features/member-add";
import { updateMember } from "@/features/member-update";
import { useAuthSession } from "@/shared/api";
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  TableCard,
} from "@/shared/ui";

const roleLabels: Record<Member["role"], string> = {
  owner: "Owner",
  admin: "Administrator",
  member: "Member",
};

function MemberList({
  members,
  actor,
  onRoleChange,
  onStatusChange,
}: {
  members: Member[];
  actor: Member | null;
  onRoleChange: (member: Member, role: Member["role"]) => void;
  onStatusChange: (member: Member) => void;
}) {
  return (
    <TableCard
      label="Member list"
      title="Workspace members"
      description={`${members.length} ${members.length === 1 ? "person" : "people"} can access this workspace`}
    >
      <div className="hidden grid-cols-[minmax(0,1fr)_7rem_11rem_7rem] gap-5 border-b px-6 py-2.5 text-xs font-semibold uppercase tracking-[0.08em] text-muted-foreground md:grid">
        <span>Member</span>
        <span>Status</span>
        <span>Role</span>
        <span className="text-right">Action</span>
      </div>
      <div className="divide-y">
        {members.map((member) => {
          const canModify = canModifyMember(actor?.role, member.role);
          const roles: Member["role"][] = actor?.role === "owner" ? ["owner", "admin", "member"] : [member.role];
          const isCurrentMember = member.id === actor?.id;

          return (
            <article
              key={member.id}
              className="grid grid-cols-[minmax(0,1fr)_minmax(8rem,1fr)] items-center gap-4 px-5 py-4 transition-colors hover:bg-muted/20 sm:px-6 md:grid-cols-[minmax(0,1fr)_7rem_11rem_7rem] md:gap-5"
            >
              <div className="col-span-2 flex min-w-0 items-center gap-3 md:col-span-1">
                <div className="flex size-10 shrink-0 items-center justify-center rounded-full bg-secondary text-sm font-semibold text-secondary-foreground" aria-hidden="true">
                  {member.name.trim().charAt(0).toUpperCase() || member.email.charAt(0).toUpperCase()}
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="truncate font-semibold">{member.name}</p>
                    {isCurrentMember ? <span className="rounded-full bg-secondary px-2 py-0.5 text-[11px] font-semibold text-muted-foreground">You</span> : null}
                  </div>
                  <p className="truncate text-sm text-muted-foreground">{member.email}</p>
                </div>
              </div>
              <div>
                <p className="mb-1.5 text-xs font-semibold uppercase tracking-[0.08em] text-muted-foreground md:hidden">Status</p>
                <MemberStatusBadge status={member.status} />
              </div>
              <div>
                <p className="mb-1.5 text-xs font-semibold uppercase tracking-[0.08em] text-muted-foreground md:hidden">Role</p>
                <Select disabled={!canModify} value={member.role} onValueChange={(role) => onRoleChange(member, role as Member["role"])}>
                  <SelectTrigger aria-label={`Role for ${member.name}`} className="h-10 rounded-lg bg-background px-3 shadow-none">
                    <SelectValue>{roleLabels[member.role]}</SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {roles.map((role) => <SelectItem key={role} value={role}>{roleLabels[role]}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="col-span-2 md:col-span-1 md:text-right">
                <p className="mb-1.5 text-left text-xs font-semibold uppercase tracking-[0.08em] text-muted-foreground md:hidden">Action</p>
                <Button className="w-full md:w-auto md:min-w-24" size="sm" variant="outline" disabled={!canModify || isCurrentMember} onClick={() => onStatusChange(member)}>
                  {member.status === "active" ? "Disable" : "Activate"}
                </Button>
              </div>
            </article>
          );
        })}
      </div>
    </TableCard>
  );
}

export function MembersPage() {
  const members = useQuery(memberQueries.list());
  const client = useQueryClient();
  const session = useAuthSession();
  const [showForm, setShowForm] = useState(false);
  const [created, setCreated] = useState<MemberCreated | null>(null);
  const canEdit = canManageMembers(session.user?.role);
  const reload = () => client.invalidateQueries({ queryKey: ["members"] });

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const values = new FormData(event.currentTarget);
    const role = String(values.get("role")) as Member["role"];
    if (!canAssignRole(session.user?.role, role)) return toast.error("You cannot assign this role");
    try {
      const result = await addMember({ name: String(values.get("name")), email: String(values.get("email")), role });
      setCreated(result);
      setShowForm(false);
      await reload();
    } catch {
      toast.error("Unable to add member", { description: "Check the email address or your account permissions, then try again.", action: { label: "Try again", onClick: () => setShowForm(true) } });
    }
  };

  const changeRole = async (member: Member, role: Member["role"]) => {
    if (!canModifyMember(session.user?.role, member.role) || !canAssignRole(session.user?.role, role)) return;
    try {
      await updateMember(member.id, { role });
      await reload();
      toast.success("Member role updated");
    } catch {
      toast.error("Unable to change role", { description: "The workspace must always have at least one active owner.", action: { label: "Try again", onClick: () => void changeRole(member, role) } });
    }
  };

  const toggleStatus = async (member: Member) => {
    if (!canModifyMember(session.user?.role, member.role)) return;
    try {
      await updateMember(member.id, { status: member.status === "active" ? "disabled" : "active" });
      await reload();
    } catch {
      toast.error("Unable to change member status", { action: { label: "Try again", onClick: () => void toggleStatus(member) } });
    }
  };

  return (
    <main className="space-y-6">
      <div className="flex justify-end">
        <Button disabled={!canEdit} onClick={() => setShowForm(true)}><Plus />Add member</Button>
      </div>
      {!canEdit ? <Card><CardContent className="pt-6 text-sm text-muted-foreground">Only administrators can edit members. Contact an administrator for help.</CardContent></Card> : null}
      {members.isError ? <Card><CardContent className="pt-6">Unable to load members. <Button variant="link" onClick={() => void members.refetch()}>Try again</Button></CardContent></Card> : null}
      <Dialog open={showForm} onOpenChange={setShowForm}>
        <DialogContent className="max-w-2xl">
          <DialogHeader><DialogTitle>New member</DialogTitle><DialogDescription>A temporary password will be created and shown once.</DialogDescription></DialogHeader>
          <form className="space-y-6" onSubmit={submit}>
            <div className="grid gap-4 md:grid-cols-2">
              <Label className="grid gap-2">Name<Input name="name" required /></Label>
              <Label className="grid gap-2">Email<Input name="email" type="email" required /></Label>
              <Label className="grid gap-2">Role<select name="role" className="h-10 rounded-md border bg-background px-3"><option value="member">Member</option>{session.user?.role === "owner" ? <><option value="admin">Administrator</option><option value="owner">Owner</option></> : null}</select></Label>
            </div>
            <DialogFooter><Button type="button" variant="outline" onClick={() => setShowForm(false)}>Cancel</Button><Button type="submit">Add member</Button></DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      {created ? <Card className="border-warning/30 bg-warning-soft/45"><CardHeader><CardTitle>Save the temporary password now</CardTitle><CardDescription>This password is shown once. Send it to {created.member.name} through a secure channel.</CardDescription></CardHeader><CardContent className="flex gap-2"><Input readOnly value={created.temporary_password} /><Button variant="outline" onClick={() => navigator.clipboard.writeText(created.temporary_password)}>Copy</Button><Button onClick={() => setCreated(null)}>Saved</Button></CardContent></Card> : null}
      {members.isSuccess && !members.data.length ? <Card><CardHeader><Users /><CardTitle>No other members yet</CardTitle></CardHeader></Card> : null}
      {members.isSuccess && members.data.length ? <MemberList members={members.data} actor={session.user} onRoleChange={(member, role) => void changeRole(member, role)} onStatusChange={(member) => void toggleStatus(member)} /> : null}
    </main>
  );
}
