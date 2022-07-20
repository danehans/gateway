package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// KindEnvoyGateway is the name of the EnvoyGateway kind.
	KindEnvoyGateway = "EnvoyGateway"
	// GatewayControllerName is the name of the GatewayClass controller.
	GatewayControllerName = "gateway.envoyproxy.io/gatewayclass-controller"
)

//+kubebuilder:object:root=true

// EnvoyGateway is the Schema for the envoygateways API.
type EnvoyGateway struct {
	metav1.TypeMeta `json:",inline"`

	// EnvoyGatewaySpec defines the desired state of Envoy Gateway.
	EnvoyGatewaySpec `json:",inline"`
}

// EnvoyGatewaySpec defines the desired state of EnvoyGateway.
type EnvoyGatewaySpec struct {
	// Gateway defines the desired Gateway API configuration. If unset,
	// default configuration parameters will apply.
	//
	// +optional
	Gateway *Gateway `json:"gateway,omitempty"`

	// Provider defines the desired provider and provider-specific configuration
	// of EnvoyGateway. If unspecified, the Kubernetes provider is used with default
	// configuration parameters.
	//
	// +optional
	Provider *EnvoyGatewayProvider `json:"provider,omitempty"`
}

// Gateway defines the desired Gateway API configuration.
type Gateway struct {
	// ControllerName defines the name of the Gateway API controller. If unspecified,
	// defaults to "gateway.envoyproxy.io/gatewayclass-controller". See the following
	// for additional details:
	//
	// https://gateway-api.sigs.k8s.io/v1alpha2/references/spec/#gateway.networking.k8s.io/v1alpha2.GatewayClass
	//
	// +optional
	ControllerName string `json:"controllerName,omitempty"`
}

// EnvoyGatewayProvider defines the desired configuration of the EnvoyGateway provider.
// +union
type EnvoyGatewayProvider struct {
	// Type is the type of provider to use.
	//
	// +unionDiscriminator
	Type ProviderType `json:"type"`
	// Kubernetes defines the configuration of the EnvoyGateway Kubernetes provider. Kubernetes
	// provides runtime configuration via the Kubernetes API.
	//
	// +optional
	Kubernetes *EnvoyGatewayKubeProvider `json:"kubernetes,omitempty"`

	// File defines the configuration of the EnvoyGateway File provider. File provides runtime
	// configuration defined by one or more files.
	//
	// +optional
	File *EnvoyGatewayFileProvider `json:"file,omitempty"`
}

// EnvoyGatewayKubeProvider defines the desired configuration for the EnvoyGateway
// Kubernetes provider.
type EnvoyGatewayKubeProvider struct {
	// TODO: Add config as use cases are better understood.
}

// EnvoyGatewayFileProvider defines configuration for the EnvoyGateway File provider.
type EnvoyGatewayFileProvider struct {
	// TODO: Add config as use cases are better understood.
}

func init() {
	SchemeBuilder.Register(&EnvoyGateway{})
}
