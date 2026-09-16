import { apiClient } from "@/shared/api";

export async function retryDelivery(deliveryId: string) {
  const { data, error } = await apiClient.POST("/v1/webhook-deliveries/{deliveryId}/retry", { params: { path: { deliveryId } } });
  if (!data) throw error;
  return data;
}
