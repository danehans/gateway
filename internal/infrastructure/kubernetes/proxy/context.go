package proxy

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KindServiceAccount Kind = "ServiceAccount"
)

type Kind string

// Resources represents all the managed Kubernetes infrastructure resources.
type Resources struct {
	serviceAccount *corev1.ServiceAccount
}

// Infra holds all the proxy infrastructure resources.
type Infra struct {
	client    client.Client
	Resources Resources
}

// NewInfra returns a new Infra.
func NewInfra(cli client.Client) *Infra {
	return &Infra{
		client: cli,
		Resources: Resources{
			serviceAccount: new(corev1.ServiceAccount),
		},
	}
}

// GetResources returns the translated proxy resources saved in the context.
func (c *Infra) GetResources() Resources {
	return c.Resources
}

// validateResource validates context resources.
func (c *Infra) validateResources() error {
	if c.Resources.serviceAccount == nil {
		return fmt.Errorf("resource kind %s is nil", KindServiceAccount)
	}
	return nil
}

// addResource adds the resource to context resources using kind to identify
// the object kind to add.
func (c *Infra) addResource(kind Kind, obj client.Object) error {
	switch kind {
	case KindServiceAccount:
		sa, ok := obj.(*corev1.ServiceAccount)
		if !ok {
			return fmt.Errorf("unexpected object kind %s", obj.GetObjectKind())
		}
		c.Resources.serviceAccount = sa
	}
	return nil
}
