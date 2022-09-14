package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/envoyproxy/gateway/internal/envoygateway/config"
	"github.com/envoyproxy/gateway/internal/ir"
	"github.com/envoyproxy/gateway/internal/utils/env"
)

// Infra holds all the translated Infra IR resources and provides
// the scaffolding for the managing Kubernetes infrastructure.
type Infra struct {
	mu     sync.Mutex
	Client client.Client
	// Namespace is the Namespace used for managed infra.
	Namespace string
	Resources *Resources
}

// Resources are managed Kubernetes resources.
type Resources struct {
	ServiceAccount *corev1.ServiceAccount
	Deployment     *appsv1.Deployment
	Service        *corev1.Service
}

// NewInfra returns a new Infra.
func NewInfra(cli client.Client) *Infra {
	infra := &Infra{
		mu:        sync.Mutex{},
		Client:    cli,
		Resources: newResources(),
	}

	// Set the namespace used for the managed infra.
	infra.Namespace = env.Lookup("ENVOY_GATEWAY_NAMESPACE", config.EnvoyGatewayNamespace)

	return infra
}

// newResources returns a new Resources.
func newResources() *Resources {
	return &Resources{
		ServiceAccount: new(corev1.ServiceAccount),
		Deployment:     new(appsv1.Deployment),
		Service:        new(corev1.Service),
	}
}

// updateResource updates the obj to the infra resources, using the object type
// to identify the object kind to add.
func (im *Infra) updateResource(obj client.Object) error {
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.Resources == nil {
		im.Resources = new(Resources)
	}

	switch o := obj.(type) {
	case *corev1.ServiceAccount:
		im.Resources.ServiceAccount = o
	case *appsv1.Deployment:
		im.Resources.Deployment = o
	case *corev1.Service:
		im.Resources.Service = o
	default:
		return fmt.Errorf("unexpected object kind %s", obj.GetObjectKind())
	}

	return nil
}

// CreateInfra creates the managed kube infra, if it doesn't exist.
func (im *Infra) CreateInfra(ctx context.Context, infra *ir.Infra) error {
	if infra == nil {
		return errors.New("infra ir is nil")
	}

	if infra.Proxy == nil {
		return errors.New("infra proxy ir is nil")
	}

	if im.Resources == nil {
		im.Resources = newResources()
	}

	if err := im.createOrUpdateServiceAccount(ctx, infra); err != nil {
		return err
	}

	if err := im.createOrUpdateDeployment(ctx, infra); err != nil {
		return err
	}

	if err := im.createOrUpdateServices(ctx, infra); err != nil {
		return err
	}

	return nil
}

// DeleteInfra removes the managed kube infra, if it doesn't exist.
func (im *Infra) DeleteInfra(ctx context.Context, infra *ir.Infra) error {
	if infra == nil {
		return errors.New("infra ir is nil")
	}

	if err := im.deleteServices(ctx); err != nil {
		return err
	}

	if err := im.deleteDeployment(ctx); err != nil {
		return err
	}

	if err := im.deleteServiceAccount(ctx); err != nil {
		return err
	}

	return nil
}
