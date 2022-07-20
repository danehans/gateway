package ir

import (
	cfgv1a1 "github.com/envoyproxy/gateway/api/config/v1alpha1"
)

// Infra defines managed infrastructure.
type Infra struct {

	// Proxy defines managed proxy infrastructure.
	Proxy *ProxyInfra
}

// ProxyInfra represents managed proxy infrastructure.
type ProxyInfra struct {
	ObjectMeta
	// Provider defines the desired provider and provider-specific configuration.
	// If unspecified, the Kubernetes provider is used with default configuration
	// parameters.
	//
	// +optional
	Provider *cfgv1a1.EnvoyGatewayProvider
	// Listeners define listener configuration.
	Listeners []HTTPListener
}
