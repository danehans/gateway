package ir

// ObjectMeta defines metadata of internal resources.
type ObjectMeta struct {
	// Name must be unique within a namespace. Name is required and is primarily intended for creation
	// idempotence and configuration definition. Name cannot be updated.
	Name string
	// Namespace defines the space within which each name must be unique. An empty namespace is equivalent
	// to the "default" namespace, but "default" is the canonical representation. Not all objects are required
	// to be scoped to a namespace - the value of this field for those objects will be empty. Must be a DNS_LABEL.
	// Cannot be updated.
	Namespace *string
	// Labels are a map of string keys and values that can be used to organize and categorize (scope and select)
	// objects.
	Labels map[string]string
}
