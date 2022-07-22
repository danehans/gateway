package ir

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true

// Infra defines managed infrastructure.
type Infra struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
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
	Namespace string
	// Image is the container image used for the managed proxy infrastructure.
	Image string
}

func init() {
	SchemeBuilder.Register(&Infra{})
}
