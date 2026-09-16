package http

import (
	"context"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// WebhookDeliveryHandler maps callback history and manual retry operations.
type WebhookDeliveryHandler struct {
	deliveries inbound.WebhookDeliveryUseCase
}

// NewWebhookDeliveryHandler binds delivery use cases.
func NewWebhookDeliveryHandler(deliveries inbound.WebhookDeliveryUseCase) *WebhookDeliveryHandler {
	return &WebhookDeliveryHandler{deliveries: deliveries}
}

// ListWebhookDeliveries returns one authorized delivery-history page.
func (h *WebhookDeliveryHandler) ListWebhookDeliveries(ctx context.Context, request gen.ListWebhookDeliveriesRequestObject) (gen.ListWebhookDeliveriesResponseObject, error) {
	page, err := h.deliveries.ListWebhookDeliveries(ctx, requestFrom(ctx).principal, request.RunId, pageRequest(request.Params.Limit, request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.WebhookDelivery, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, webhookDeliveryDTO(item))
	}
	return gen.ListWebhookDeliveries200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// RetryWebhookDelivery schedules a failed callback immediately.
func (h *WebhookDeliveryHandler) RetryWebhookDelivery(ctx context.Context, request gen.RetryWebhookDeliveryRequestObject) (gen.RetryWebhookDeliveryResponseObject, error) {
	item, err := h.deliveries.RetryWebhookDelivery(ctx, requestFrom(ctx).principal, request.DeliveryId)
	if err != nil {
		return nil, err
	}
	return gen.RetryWebhookDelivery200JSONResponse(webhookDeliveryDTO(item)), nil
}

func webhookDeliveryDTO(value domain.WebhookDelivery) gen.WebhookDelivery {
	return gen.WebhookDelivery{Id: value.ID, RunId: value.RunID, ApiKeyId: value.APIKeyID, Url: value.URL, Event: gen.WebhookDeliveryEvent(value.Event), Status: gen.WebhookDeliveryStatus(value.Status), Attempts: value.Attempts, NextAttemptAt: nullableValue(value.NextAttemptAt), LastStatusCode: nullableValue(value.LastStatusCode), LastError: nullableValue(value.LastError), DeliveredAt: nullableValue(value.DeliveredAt), CreatedAt: value.CreatedAt}
}
