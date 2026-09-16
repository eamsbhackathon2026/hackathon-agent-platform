import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { describe, expect, it, vi } from "vitest";

import type { WebhookDelivery } from "@/entities/run";
import { server } from "@/test/msw-server";
import { jsonResponse } from "@/test/typed-handlers";

import { WebhookDeliveryPanel } from "./webhook-delivery-panel";

const delivery: WebhookDelivery = {
  id: "delivery-1", run_id: "run-1", api_key_id: "key-1", url: "https://example.test/hook",
  event: "run.failed", status: "failed", attempts: 8, next_attempt_at: null, last_status_code: 500,
  last_error: "The destination server did not respond", delivered_at: null, created_at: "2026-09-15T00:00:00Z",
};

describe("WebhookDeliveryPanel", () => {
  it("retries the selected failed delivery", async () => {
    const retried = vi.fn();
    server.use(http.post("*/v1/webhook-deliveries/:deliveryId/retry", ({ params }) => {
      retried(params.deliveryId);
      return jsonResponse({ ...delivery, status: "pending", attempts: 0 });
    }));
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    render(<QueryClientProvider client={client}><WebhookDeliveryPanel runId="run-1" deliveries={[delivery]} /></QueryClientProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(retried).toHaveBeenCalledWith("delivery-1"));
  });
});
