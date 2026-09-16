import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/shared/ui";

type ActiveRunNavigationDialogProps = {
  open: boolean;
  busy: boolean;
  onKeepWaiting: () => void;
  onStopAndSwitch: () => void;
  onRestoreFocus: () => void;
};

export function ActiveRunNavigationDialog({ open, busy, onKeepWaiting, onStopAndSwitch, onRestoreFocus }: ActiveRunNavigationDialogProps) {
  return <AlertDialog open={open}>
    <AlertDialogContent onCloseAutoFocus={(event) => { event.preventDefault(); onRestoreFocus(); }}>
      <AlertDialogHeader>
        <AlertDialogTitle>Stop the current response?</AlertDialogTitle>
        <AlertDialogDescription>The assistant is still responding. Keep waiting, or stop the activity before switching conversations.</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel disabled={busy} onClick={onKeepWaiting}>Keep waiting</AlertDialogCancel>
        <AlertDialogAction disabled={busy} onClick={(event) => { event.preventDefault(); onStopAndSwitch(); }}>{busy ? "Stopping…" : "Stop and switch"}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>;
}
