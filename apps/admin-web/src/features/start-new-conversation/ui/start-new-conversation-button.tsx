import { MessageSquarePlus } from "lucide-react";

import { Button } from "@/shared/ui";

export function StartNewConversationButton({ onStart }: { onStart: () => void }) {
  return <Button type="button" variant="outline" onClick={onStart}><MessageSquarePlus />New conversation</Button>;
}
