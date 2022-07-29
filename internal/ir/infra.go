package ir

import (
	"errors"
	"fmt"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/envoyproxy/gateway/api/config/v1alpha1"
)

const (
	DefaultProxyName      = "default"
	DefaultProxyNamespace = "default"
	DefaultProxyImage     = "envoyproxy/envoy-dev:latest"
)

// Infra defines managed infrastructure.
type Infra struct {
	// Provider is the provider of the infrastructure.
	Provider *v1alpha1.ProviderType
	// Proxy defines managed proxy infrastructure.
	Proxy *ProxyInfra
}

// ProxyInfra defines managed proxy infrastructure.
type ProxyInfra struct {
	// TODO: Figure out how to represent metadata in the IR.
	// xref: https://github.com/envoyproxy/gateway/issues/173
	//
	// Name is the name used for managed proxy infrastructure.
	Name string
	// Namespace is the namespace used for managed proxy infrastructure.
	// If unset, defaults to "default".
	Namespace string
	// Config defines user-facing configuration of the managed proxy infrastructure.
	Config *v1alpha1.EnvoyProxy
	// Image is the container image used for the managed proxy infrastructure.
	// If unset, defaults to "envoyproxy/envoy-dev:latest".
	Image string
}

// NewInfra returns a new Infra with default parameters.
func NewInfra() *Infra {
	return &Infra{
		// Kube is the only supported provider type.
		Provider: v1alpha1.ProviderTypePtr(v1alpha1.ProviderTypeKubernetes),
		Proxy:    NewProxyInfra(),
	}
}

// NewProxyInfra returns a new ProxyInfra with default parameters.
func NewProxyInfra() *ProxyInfra {
	return &ProxyInfra{
		Name:      DefaultProxyName,
		Namespace: DefaultProxyNamespace,
		Image:     DefaultProxyImage,
	}
}

// GetProvider returns the infra provider.
func (i *Infra) GetProvider() *v1alpha1.ProviderType {
	if i.Provider != nil {
		return i.Provider
	}
	// Kube is the default infra provider.
	return v1alpha1.ProviderTypePtr(v1alpha1.ProviderTypeKubernetes)
}

// GetProxyInfra returns the ProxyInfra.
func (i *Infra) GetProxyInfra() *ProxyInfra {
	if i.Proxy == nil {
		return NewProxyInfra()
	}
	p := new(ProxyInfra)
	if len(i.Proxy.Namespace) == 0 {
		p.Namespace = DefaultProxyNamespace
	}
	if len(i.Proxy.Name) == 0 {
		p.Name = DefaultProxyName
	}
	if len(i.Proxy.Image) == 0 {
		p.Image = DefaultProxyImage
	}

	return p
}

// ValidateInfra validates the provided Infra.
func ValidateInfra(infra *Infra) error {
	if infra == nil {
		return errors.New("infra ir is nil")
	}

	var errs []error

	if infra.Proxy != nil {
		if err := ValidateProxyInfra(infra.Proxy); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

// ValidateProxyInfra validates the provided ProxyInfra.
func ValidateProxyInfra(pInfra *ProxyInfra) error {
	var errs []error

	if len(pInfra.Name) == 0 {
		errs = append(errs, errors.New("name field required"))
	}

	if len(pInfra.Namespace) == 0 {
		errs = append(errs, errors.New("namespace field required"))
	}

	if len(pInfra.Image) == 0 {
		errs = append(errs, errors.New("image field required"))
	}

	return utilerrors.NewAggregate(errs)
}

// ObjectName returns the name of proxy infrastructure objects.
func (p *ProxyInfra) ObjectName() string {
	if len(p.Name) == 0 {
		return fmt.Sprintf("envoy-%s", DefaultProxyName)
	}
	return "envoy-" + p.Name
}
