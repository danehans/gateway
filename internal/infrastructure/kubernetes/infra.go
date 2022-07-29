package kubernetes

import (
	"fmt"
	"github.com/envoyproxy/gateway/internal/infrastructure"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KindServiceAccount Kind = "ServiceAccount"
)

type Kind string

// Infra holds all the translated Infra IR resources and provides
// the scaffolding for the managing Kubernetes infrastructure.
type Infra struct {
	infrastructure.Manager
	Client    client.Client
	Resources *Resources
}

// Resources are managed Kubernetes resources.
type Resources struct {
	ServiceAccount *corev1.ServiceAccount
}

// NewInfra returns a new Infra.
func NewInfra(cli client.Client) *Infra {
	return &Infra{
		Client: cli,
		Resources: &Resources{
			ServiceAccount: new(corev1.ServiceAccount),
		},
	}
}

// addResource adds the resource to the infra resources, using kind to
// identify the object kind to add.
func (i *Infra) addResource(kind Kind, obj client.Object) error {
	if i.Resources == nil {
		i.Resources = new(Resources)
	}

	switch kind {
	case KindServiceAccount:
		sa, ok := obj.(*corev1.ServiceAccount)
		if !ok {
			return fmt.Errorf("unexpected object kind %s", obj.GetObjectKind())
		}
		i.Resources.ServiceAccount = sa
	default:
		return fmt.Errorf("unexpected object kind %s", obj.GetObjectKind())
	}

	return nil
}
