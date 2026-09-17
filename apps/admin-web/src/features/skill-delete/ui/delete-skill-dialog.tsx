import type { MouseEvent } from "react";
import { useState } from "react";
import { toast } from "sonner";

import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/shared/ui";

import { deleteSkill } from "../api/delete-skill";

type DeleteSkillDialogProps = {
  skill: { id: string; name: string } | null;
  onOpenChange: (open: boolean) => void;
  onDeleted?: (() => void) | undefined;
};

/**
 * The single confirmation step for deleting a skill. The caller owns which skill
 * is pending, so the dialog survives the card it was opened from.
 */
export function DeleteSkillDialog({ skill, onOpenChange, onDeleted }: DeleteSkillDialogProps) {
  const [deleting, setDeleting] = useState(false);

  // Closing on the same click that starts the request would leave the card and
  // its Delete button live while the call is still in flight. Holding the dialog
  // open until it settles keeps that window shut.
  const confirm = async (event: MouseEvent) => {
    event.preventDefault();
    if (!skill || deleting) return;
    setDeleting(true);
    try {
      await deleteSkill(skill.id);
      onDeleted?.();
      toast.success("Skill deleted");
      onOpenChange(false);
    } catch (error) {
      toast.error("Unable to delete skill", { description: deleteMessage(error) });
    } finally {
      setDeleting(false);
    }
  };

  return <AlertDialog open={Boolean(skill)} onOpenChange={(open) => { if (!deleting) onOpenChange(open); }}>
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>Delete “{skill?.name}”?</AlertDialogTitle>
        <AlertDialogDescription>This skill will be removed from every assistant that follows it. Assistants keep working, but they will no longer receive these instructions.</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel disabled={deleting}>Keep it</AlertDialogCancel>
        <AlertDialogAction disabled={deleting} onClick={(event) => void confirm(event)}>{deleting ? "Deleting…" : "Delete skill"}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>;
}

function deleteMessage(error: unknown) {
  if (typeof error === "object" && error !== null && "detail" in error && typeof error.detail === "string") return error.detail;
  if (error instanceof Error && error.message) return error.message;
  return "Try again.";
}
