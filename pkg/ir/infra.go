package ir

import (
	cfgv1a1 "github.com/envoyproxy/gateway/api/config/v1alpha1"
)

// InfraContext defines the managed infrastructure.
type InfraContext struct {
	// Proxy defines the managed proxy infrastructure.
	Proxy *ProxyContext
}

// ProxyContext represents one or more managed proxies.
type ProxyContext struct {
	ObjectMeta
	// Config define proxy configuration.
	Config *cfgv1a1.EnvoyProxy
	// Listeners define listener configuration.
	Listeners []HTTPListener
	// Service defines the...
	Service *Service
	// Secret defines the...
	Secret *Secret
	// Deployment defines the...
	Deployment *Deployment
}

// Service represented a Kubernetes Service resource.
type Service struct {
	ObjectMeta
	// Route service traffic to pods with label keys and values matching this selector.
	Selector map[string]string
	Ports    []*ServicePort
}

// ServicePort represents a Kubernetes ServicePort resource.
type ServicePort struct {
	Name       string
	Port       int32
	TargetPort int32
	NodePort   *int32
}

// Secret represented a Kubernetes Secret resource.
type Secret struct {
	Namespace string
	Name      string
	// Data contains the secret data. Each key must consist of alphanumeric characters, '-', '_' or '.'.
	Data map[string][]byte
}

// Deployment represented a Kubernetes Deployment resource.
type Deployment struct {
}
