import { useMutation, useQueryClient } from "@tanstack/react-query";
import { RotateCcw } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/shared/ui";
import { retryDelivery } from "../api/retry-delivery";

export function RetryDeliveryButton({ deliveryId, runId }: { deliveryId: string; runId: string }) {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: () => retryDelivery(deliveryId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["runs", runId, "deliveries"] });
      toast.success("Retry scheduled");
    },
    onError: () => toast.error("Unable to retry delivery"),
  });
  return <Button type="button" size="sm" variant="outline" disabled={mutation.isPending} onClick={() => mutation.mutate()}><RotateCcw />Retry</Button>;
}
