package kubernetes

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/envoyproxy/gateway/internal/envoygateway"
	"github.com/envoyproxy/gateway/internal/envoygateway/config"
	"github.com/envoyproxy/gateway/internal/infrastructure/kubernetes/proxy"
)

// Manager is the scaffolding for the Kubernetes infra manager.
type Manager struct {
	client  client.Client
	runtime manager.Manager
	infra   *Infra
}

// Infra holds all the managed infrastructure resources.
type Infra struct {
	proxy *proxy.Infra
}

// NewManager creates a new Manager from the provided restCfg and svrCfg.
func NewManager(restCfg *rest.Config, svrCfg *config.Server) (*Manager, error) {
	mgrOpts := manager.Options{
		Scheme:             envoygateway.GetScheme(),
		Logger:             svrCfg.Logger,
		LeaderElection:     false,
		LeaderElectionID:   "5b9825d2.gateway.envoyproxy.io",
		MetricsBindAddress: ":8080",
	}
	mgr, err := ctrl.NewManager(restCfg, mgrOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime manager: %w", err)
	}

	// Create and register the controller with the runtime manager.
	if err := newController(mgr, svrCfg); err != nil {
		return nil, fmt.Errorf("failed to create infra controller: %w", err)
	}

	return &Manager{
		client:  mgr.GetClient(),
		runtime: mgr,
		infra:   newInfra(mgr.GetClient(), svrCfg),
	}, nil
}

// newInfra returns a new Infra.
func newInfra(cli client.Client, cfg *config.Server) *Infra {
	return &Infra{
		proxy: proxy.NewInfra(cli, cfg),
	}
}

// Start starts the Manager synchronously until a message is received from ctx.
func (m *Manager) Start(ctx context.Context) error {
	errChan := make(chan error)
	go func() {
		errChan <- m.runtime.Start(ctx)
	}()

	// Wait for the runtime to exit or an explicit stop.
	select {
	case <-ctx.Done():
		return nil
	case err := <-errChan:
		return err
	}
}
