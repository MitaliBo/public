// +build go1.7

package supervisor

import (
	"context"
	"time"
)

var (
	defaultContext = context.Background()
)

// Add inserts new service into the DefaultSupervisor. If the DefaultSupervisor
// is already started, it will start it automatically. If the same service is
// added more than once, it will reset its backoff mechanism and force a service
// restart.
func Add(service Service) {
	DefaultSupervisor.Add(service)
}

// Cancelations return a list of services names of DefaultSupervisor and their
// cancelation calls. These calls be used to force a service restart.
func Cancelations() map[string]context.CancelFunc {
	return DefaultSupervisor.Cancelations()
}

// ServeContext starts the DefaultSupervisor tree with a custom context.Context.
// It can be started only once at a time. If stopped (canceled), it can be
// restarted. In case of concurrent calls, it will hang until the current call
// is completed.
func ServeContext(ctx context.Context) {
	DefaultSupervisor.Serve(ctx)
}