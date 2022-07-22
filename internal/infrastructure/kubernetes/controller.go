package kubernetes

import (
	"context"
	"github.com/envoyproxy/gateway/internal/envoygateway/config"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type reconciler struct {
	client client.Client
	log    logr.Logger
	// TODO: Add map/channel to consume infra ir.
	source chan event.GenericEvent
}

// newController creates a controller that triggers reconciliation for watched events.
func newController(mgr manager.Manager, cfg *config.Server) error {
	cli := mgr.GetClient()

	r := &reconciler{
		client: cli,
		log:    cfg.Logger,
		source: make(chan event.GenericEvent),
	}

	c, err := controller.New("infra", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	r.log.Info("created infra controller")

	// Only enqueue GatewayClass objects that match this Envoy Gateway's controller name.
	if err := c.Watch(
		&source.Channel{Source: r.source,
		},
		&handler.EnqueueRequestForObject{},
	); err != nil {
		return err
	}
	r.log.Info("watching gatewayclass objects")

	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	r.log.WithName(request.Name).Info("reconciling gatewayclass")

	i := new(Infra)
	r.source <- event.GenericEvent{Object: i}

	return reconcile.Result{}, nil
}
