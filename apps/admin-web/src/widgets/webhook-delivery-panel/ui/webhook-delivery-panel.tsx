import type { WebhookDelivery } from "@/entities/run";
import { RetryDeliveryButton } from "@/features/retry-webhook-delivery";
import { formatDate } from "@/shared/lib";
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/shared/ui";

const statusLabel = { pending: "Pending", delivered: "Delivered", failed: "Failed" } as const;

export function WebhookDeliveryPanel({ runId, deliveries }: { runId: string; deliveries: WebhookDelivery[] }) {
  return <Card><CardHeader><CardTitle>Deliver results to another system</CardTitle><CardDescription>Delivery history for the endpoint configured with this access key.</CardDescription></CardHeader><CardContent className="space-y-3">
    {!deliveries.length ? <p className="text-sm text-muted-foreground">This activity has no delivery endpoint.</p> : deliveries.map((delivery) => <div key={delivery.id} className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3 text-sm">
      <div className="min-w-0"><div className="flex items-center gap-2"><Badge variant={delivery.status === "failed" ? "destructive" : "outline"}>{statusLabel[delivery.status]}</Badge><span>{delivery.attempts} attempts</span></div><p className="mt-1 truncate text-xs text-muted-foreground">{delivery.url} · {formatDate(delivery.created_at)}</p>{delivery.last_error ? <p className="mt-1 text-xs text-destructive">{delivery.last_error}</p> : null}</div>
      {delivery.status === "failed" ? <RetryDeliveryButton deliveryId={delivery.id} runId={runId} /> : null}
    </div>)}
  </CardContent></Card>;
}
