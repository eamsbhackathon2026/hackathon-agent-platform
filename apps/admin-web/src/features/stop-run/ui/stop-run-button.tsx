import { Square } from "lucide-react";

import { Button } from "@/shared/ui";

export function StopRunButton({ onStop, disabled }: { onStop: () => void; disabled?: boolean }) {
  return <Button type="button" variant="outline" disabled={disabled} onClick={onStop}><Square />Stop</Button>;
}
