package main

import (
	"fmt"
	"log/slog"
	"os"

	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	workeradapter "agent-platform/services/agent-service/internal/adapters/inbound/worker"
	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	webhookadapter "agent-platform/services/agent-service/internal/adapters/outbound/webhook"
	"agent-platform/services/agent-service/internal/config"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/core/services/delivery"
	"agent-platform/services/agent-service/internal/core/services/runs"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

func wireAsync(cfg config.Config, store *postgres.Store, cipher outbound.SecretCipher, realClock outbound.Clock, runService *runs.Service, guard *netguard.Guard, log *slog.Logger) (*httpadapter.WebhookDeliveryHandler, *workeradapter.Group, error) {
	sender := webhookadapter.NewHTTPSender(guard.NewHTTPClient())
	dispatcher, err := delivery.NewDispatcher(delivery.DispatcherDependencies{Deliveries: store, APIKeys: store, Cipher: cipher, Signer: webhookadapter.StandardWebhooksSigner{}, Sender: sender, Clock: realClock, URLs: guard})
	if err != nil {
		return nil, nil, err
	}
	deliveryService, err := delivery.NewService(delivery.ServiceDependencies{Deliveries: store, Runs: store, APIKeys: store, URLs: guard, Clock: realClock})
	if err != nil {
		return nil, nil, err
	}
	workerID := fmt.Sprintf("%s-%d", hostname(), os.Getpid())
	group := &workeradapter.Group{
		RunWorker:         &workeradapter.RunWorker{Queue: store, Processor: runService, Logger: log, WorkerID: workerID, Concurrency: cfg.WorkerConcurrency},
		WebhookWorker:     &workeradapter.WebhookWorker{Dispatcher: dispatcher, Logger: log},
		MaintenanceWorker: &workeradapter.MaintenanceWorker{Store: store, Clock: realClock, Logger: log},
	}
	return httpadapter.NewWebhookDeliveryHandler(deliveryService), group, nil
}

func hostname() string {
	value, err := os.Hostname()
	if err != nil || value == "" {
		return "agent-service"
	}
	return value
}
