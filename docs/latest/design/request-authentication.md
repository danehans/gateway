# Request Authentication

## Overview

[Issue 336][] specifies the need for exposing a user-facing API to configure request authentication. Request
authentication is defined as an authentication mechanism to be enforced by Envoy on a per-request basis. A connection
will be rejected if it contains invalid authentication information, based on the `Authentication` API type proposed in
this design document.

Envoy Gateway leverages [Gateway API][] for configuring managed Envoy proxies. Gateway API defines core, extended, and
implementation-specific API [support levels][] for implementors such as Envoy Gateway to expose features. Since
implementing request authentication is not covered by `Core` or `Extended` APIs, an `Implementation-specific` API will
be created for this purpose.

## Goals

* Define an API for configuring request authentication.
* Implement [JWT] as the first supported authentication type.
* Allow users that manage routes, e.g. [HTTPRoute][], to authenticate matching requests before forwarding to a backend
  service.
* Support HTTPRoutes as an Authentication API referent.

## Non-Goals

* Allow infrastructure administrators to override or establish default authentication policies.
* Support referents other than HTTPRoute.

## Use-Cases

These use-cases are presented as an aid for how users may attempt to utilize the outputs of the design. They are not an
exhaustive list of features for authentication support in Envoy Gateway.

As a Service Producer, I need the ability to:
* Authenticate a request before forwarding it to a backend service.
* Have different authentication mechanisms per route rule.
* Choose from different authentication mechanisms supported by Envoy, e.g. OIDC.

### Security API Group

A new API group, `security.gateway.envoyproxy.io` is introduced to group security-related APIs. This will allow security
APIs to be versioned, changed, etc. over time.

### Authentication API Type

The Authentication API type defines authentication configuration for authenticating requests through managed Envoy
proxies.

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

)

type Authentication struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec defines the desired state of the Authentication type.
	Spec AuthenticationSpec

	// Note: The status sub-resource has been excluded but may be added in the future.
}

// AuthenticationSpec defines the desired state of the Authentication type.
// +union
type AuthenticationSpec struct {
	// Type defines the type of authentication provider to use. Supported provider
	// types are:
	//
	// * JWT
	//
	//   JWT defines the JSON Web Token (JWT) authentication provider type.
	//
	// +unionDiscriminator
	Type AuthenticationType `json:"type"`
	
	// JWT defines the JSON Web Token (JWT) authentication provider type. When multiple
	// jwtProviders are specified, the JWT is considered valid if any of the providers
	// successfully validate the JWT. For additional details, see:
	//
	//   https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/jwt_authn_filter.html
	//
	// +kubebuilder:validation:MaxItems=4
	// +optional
	JwtProviders []JwtAuthenticationProvider `json:"jwtProviders,omitempty"`
}

// AuthenticationType is a type of authentication provider.
// +kubebuilder:validation:Enum=JWT
type AuthenticationType string

const (
	// JwtAuthenticationProviderType is the JWT authentication provider type.
	JwtAuthenticationProviderType AuthenticationType = "JWT"
)

// JwtAuthenticationProvider defines the JSON Web Token (JWT) authentication provider type
// and how JWTs should be verified:
type JwtAuthenticationProvider struct {
	// Name defines a unique name for the JWT provider. A name can have a variety of forms,
	// including RFC1123 subdomains, RFC 1123 labels, or RFC 1035 labels.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Issuer is the principal that issued the JWT.	For additional details, see:
	//
	//   https://tools.ietf.org/html/rfc7519#section-4.1.1
	//
	// Example:
	//  issuer: https://auth.example.com
	//
	// If not provided, the JWT issuer is not checked.
	//
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Issuer string `json:"issuer,omitempty"`

	// Audiences is a list of JWT audiences allowed to access. For additional details, see:
	//
	//   https://tools.ietf.org/html/rfc7519#section-4.1.3
	//
	// Example:
	//   audiences:
	//   - foo.apps.example.com
	//     bar.apps.example.com
	//
	// If not provided, JWT audiences are not checked.
	//
	// +kubebuilder:validation:MaxItems=8
	// +optional
	Audiences []string `json:"audiences,omitempty"`

	// RemoteJWKS defines how to fetch and cache JSON Web Key Sets (JWKS) from a remote
	// HTTP/HTTPS endpoint.
	RemoteJWKS RemoteJWKS `json:"remoteJWKS"`

	// TODO: Add TBD JWT fields based on defined use cases.
}

// RemoteJWKS defines how to fetch and cache JSON Web Key Sets (JWKS) from a remote
// HTTP/HTTPS endpoint.
type RemoteJWKS struct {
	// Uri is the HTTP/HTTPS URI to fetch the JWKS.
	//
	// Example:
	//  uri: https://www.googleapis.com/oauth2/v1/certs
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Uri string `json:"uri"`

	// TODO: Add TBD remote JWKS fields based on defined use cases.
}
```

The status subresource is not included in the Authentication API. Status will be surfaced by an HTTPRoute that
references an Authentication. For example, an HTTPRoute will surface the `ResolvedRefs=False` status condition if it
references an Authentication that does not exist. It may be beneficial to add status fields in the future based on
defined use-cases. For example, a remote JWKS can be validated based on the specified URI and have an appropriate
status condition surfaced.

The following is an example of a JWT authentication provider:

```yaml
apiVersion: security.gateway.envoyproxy.io/v1alpha1
kind: Authentication
metadata:
  name: example-jwt
spec:
  type: JWT
  jwtProviders:
  - name: example
    issuer: https://www.example.com
    audiences:
    - foo.com
    remoteJwks:
      uri: https://foo.com/jwt/public-key/jwks.json
      <TBD>
status:
  <TBD>
```

The following is an example HTTPRoute configured to use the above JWT authentication provider:

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: example-hwt-authn
spec:
  parentRefs:
    - name: eg
  hostnames:
    - www.example.com
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /foo
      filters:
        - type: ExtensionRef
          extensionRef:
            group: security.gateway.envoyproxy.io
            kind: Authentication
            name: example-jwt
      backendRefs:
        - name: httpbin
          port: 80
```

Requests for `www.example.com/foo` will be authenticated using the referenced JWT provider before being forwarded to the
`httpbin` backend service.

## Implementation Details

The JWT authentication type is translated to an Envoy [JWT authentication filter][] and a cluster is created for each
remote JWKS. The following cluster is created for the JWT provider defines in the above Authentication:

```yaml
clusters:
  - name: foo.com|443
    load_assignment:
      cluster_name: foo.com|443
      endpoints:
        - lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    address: foo.com
                    port_value: 443
    transport_socket:
      name: envoy.transport_sockets.tls
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext
        sni: foo.com
        common_tls_context:
          validation_context:
            match_subject_alt_names:
              - exact: "*.foo.com"
            trusted_ca:
              filename: /etc/ssl/certs/ca-certificates.crt
```

A JWT authentication HTTP filter is added to the HTTP Connection Manager. For example:

```yaml
dynamic_resources:
  dynamic_listeners:
    - name: example_listener
      address:
        socket_address:
          address: 1.2.3.4
          port_value: 80
      filter_chains:
        - filters:
            - name: envoy.http_connection_manager
          http_filters:
          - name: envoy.filters.http.jwt_authn
            typed_config:
              "@type": type.googleapis.com/envoy.config.filter.http.jwt_authn.v2alpha.JwtAuthentication
```

This JWT authentication filter contains two fields:
* The `providers` field specifies how a JWT should be verified, such as where to extract the token, where to fetch the
  public key (JWKS) and where to output its payload. This field is built from `jwtProviders` of an Authentication.
* The `rules` field specifies matching rules and their requirements. If a request matches a rule, its requirement
  applies. The requirement specifies which JWT providers should be used. This field is built from a HTTPRoute
  `matches` rule that references the Authentication extended filter. When a referenced Authentication specifies
  multiple `jwtProviders`, the JWT is considered valid if __any__ of the providers successfully validate the JWT.

The following JWT Authentication filter `providers` configuration is created from the above Authentication.

```yaml
providers:
   example:
     issuer: https://www.example.com
     audiences:
     - foo.com
     remote_jwks:
       http_uri:
         uri: https://foo.com/jwt/public-key/jwks.json
         cluster: example_jwks_cluster
         timeout: 1s
```

The following JWT Authentication filter `rules` configuration is created from the above HTTPRoute.

```yaml
rules:
  - match:
      prefix: /foo
    requires:
      provider_name: example
```

If the Authentication included a second JWT provider named `example`, the resulting JWT Authentication filter `rules`
would be created:

```yaml
rules:
- match:
    prefix: /foo
  requires:
    requires_any:
      requirements:
      - provider_name: example
      - provider_name: example2
```

### Implementation Outline

* Create a `security` API group and add the Authentication API type to this group.
* Update the Kubernetes provider to get/watch Authentication resources that are referenced by managed HTTPRoutes. Add
  the referenced Authentication object to the resource map and publish it.
* Update the resource translator to include the Authentication API in HTTPRoute processing.
* Update the xDS translator to translate an Authentication into xDS resources. The translator should perform the
  following:
  * Convert a list of JWT rules from the xds IR into an Envoy JWT filter config.
  * Create a JWT authentication filter.
  * Build the HTTP Connection Manager (MCM) HTTP filters.
  * Build the HCM.
  * When building the Listener, create an HCM for each filter-chain.

## Adding Authentication Provider Types

Additional authentication provider types can be added in the future through the `ProviderType` API. For example, to add
the `Foo` authentication provider type:

Define the `FooProvider` type:

```go
package v1alpha1

// FooProvider defines the "Foo" authentication provider type.
type FooProvider struct {
	// TODO: Define fields of the Foo authentication provider type.
}
```

Add the `FooProvider` type to `AuthenticationSpec`:

```go
package v1alpha1

type AuthenticationSpec struct {
	...
	
	// Foo defines the Foo authentication type. For additional
	// details, see:
	//
	//   <INSERT_LINK>
	//
	// +optional
	Foo *FooAuthentication `json:"foo"`
}
```

Authentication should support additional authentication types in the future, for example:
- mutualTLS (client certificate)
- OAuth2
- OIDC
- External authentication

## Outstanding Questions

- If Envoy Gateway owns the Authentication API, is an xDS IR equivalent needed?
- Should local JWKS be implemented before remote JWKS?
- How should Envoy obtain the trusted CA for a remote JWKS?
- Should HTTPS be the only supported scheme for remote JWKS?
- Should OR'ing JWT providers be supported?
- Should Authentication provide status?
- Are the API field validation rules acceptable?

[Issue 336]: https://github.com/envoyproxy/gateway/issues/336
[Gateway API]: https://gateway-api.sigs.k8s.io/
[support levels]: https://gateway-api.sigs.k8s.io/concepts/conformance/?h=extended#2-support-levels
[JWT]: https://jwt.io/
[HTTPRoute]: https://gateway-api.sigs.k8s.io/api-types/httproute/
[JWKS]: https://www.rfc-editor.org/rfc/rfc7517
[JWT authentication filter]: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/jwt_authn_filter#config-http-filters-jwt-authn