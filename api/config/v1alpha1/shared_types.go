package v1alpha1

// ObjectMeta defines metadata of internal resources.
type ObjectMeta struct {
	// Name must be unique within a namespace. Name is required and is primarily intended for creation
	// idempotence and configuration definition. Name cannot be updated.
	Name string `json:"name"`
	// Namespace defines the space within which each name must be unique. An empty namespace is equivalent
	// to the "default" namespace, but "default" is the canonical representation. Not all objects are required
	// to be scoped to a namespace - the value of this field for those objects will be empty. Must be a DNS_LABEL.
	// Cannot be updated.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Labels are a map of string keys and values that can be used to organize and categorize (scope and select)
	// objects.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// ProviderType defines the types of providers supported by Envoy Gateway.
type ProviderType string

const (
	// ProviderTypeKubernetes defines the "Kubernetes" provider.
	ProviderTypeKubernetes ProviderType = "Kubernetes"

	// ProviderTypeFile defines the "File" provider.
	ProviderTypeFile ProviderType = "File"
)
