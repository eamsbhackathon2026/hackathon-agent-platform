package worker

import (
	"context"
	"sync"
)

// Group owns all background loops started by the service process.
type Group struct {
	RunWorker         *RunWorker
	WebhookWorker     *WebhookWorker
	MaintenanceWorker *MaintenanceWorker
}

// Run starts configured workers and waits for shutdown.
func (g *Group) Run(ctx context.Context) {
	var group sync.WaitGroup
	for _, run := range []func(context.Context){g.RunWorker.Run, g.WebhookWorker.Run, g.MaintenanceWorker.Run} {
		group.Add(1)
		go func(run func(context.Context)) { defer group.Done(); run(ctx) }(run)
	}
	group.Wait()
}
