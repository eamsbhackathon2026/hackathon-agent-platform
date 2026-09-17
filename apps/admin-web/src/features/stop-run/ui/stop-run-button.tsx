import { Square } from "lucide-react";

import { Button } from "@/shared/ui";

export function StopRunButton({ onStop, disabled }: { onStop: () => void; disabled?: boolean }) {
  return <Button type="button" size="icon" variant="outline" className="rounded-full" aria-label="Stop" title="Stop" disabled={disabled} onClick={onStop}><Square /></Button>;
}
