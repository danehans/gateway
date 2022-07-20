package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EnvoyProxy is the Schema for the envoyproxies API.
type EnvoyProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvoyProxySpec   `json:"spec,omitempty"`
	Status EnvoyProxyStatus `json:"status,omitempty"`
}

// EnvoyProxySpec defines the desired state of EnvoyProxy.
type EnvoyProxySpec struct {
	// Provider defines the desired provider and provider-specific configuration
	// of EnvoyProxy. If unspecified, the Kubernetes provider is used with default
	// configuration parameters.
	//
	// +optional
	Provider *ProxyProvider `json:"provider,omitempty"`
}

// ProxyProvider defines the desired configuration of the EnvoyProxy provider.
// +union
type ProxyProvider struct {
	// Type is the type of EnvoyProxy provider to use.
	//
	// +unionDiscriminator
	Type ProviderType `json:"type"`
	// Kubernetes defines the configuration of the EnvoyProxy Kubernetes provider. Kubernetes
	// provides runtime configuration via the Kubernetes API.
	//
	// +optional
	Kubernetes *ProxyKubeProvider `json:"kubernetes,omitempty"`
}

// ProxyKubeProvider defines the desired configuration for the EnvoyProxy
// Kubernetes provider.
type ProxyKubeProvider struct {
	// Service defines configuration of a Kubernetes Service resource.
	//
	// +optional
	Service *KubeService `json:"service,omitempty"`
	// Deployment defines configuration of a Kubernetes Deployment resource.
	//
	// +optional
	Deployment *KubeDeployment `json:"deployment,omitempty"`
}

// KubeService defines configuration of a Kubernetes Service resource.
type KubeService struct {
	// Type is the Service type. For additional details, see:
	// https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	//
	// +unionDiscriminator
	Type KubeServiceType `json:"type"`
}

// KubeServiceType defines the Kubernetes service types supported by Envoy Gateway.
type KubeServiceType string

const (
	// ClusterIPKubeServiceType defines the Kubernetes "ClusterIP" service type.
	ClusterIPKubeServiceType KubeServiceType = "ClusterIP"

	// LoadBalancerKubeServiceType defines the Kubernetes "LoadBalancer" service type.
	LoadBalancerKubeServiceType KubeServiceType = "LoadBalancer"
)

// KubeDeployment configuration of a Kubernetes Deployment resource.
type KubeDeployment struct {
	// Replicas defines the number of desired pods. This is a pointer to distinguish
	// between explicit zero and unspecified. Defaults to 1.
	//
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`
}

// EnvoyProxyStatus defines the observed state of EnvoyProxy
type EnvoyProxyStatus struct {
	// INSERT ADDITIONAL STATUS FIELDS - define observed state of cluster.
	// Important: Run "make" to regenerate code after modifying this file.
}

//+kubebuilder:object:root=true

// EnvoyProxyList contains a list of EnvoyProxy
type EnvoyProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnvoyProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EnvoyProxy{}, &EnvoyProxyList{})
}
