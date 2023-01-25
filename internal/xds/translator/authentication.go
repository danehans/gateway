// Copyright Envoy Gateway Authors
// SPDX-License-Identifier: Apache-2.0
// The full text of the Apache license is available in the LICENSE file at
// the root of the repo.

package translator

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	jwtext "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/envoyproxy/gateway/internal/ir"
)

const jwtAuthenFilter = "envoy.filters.http.jwt_authn"

// patchHCMWithJwtAuthnFilter builds and appends the Jwt Filter to the HTTP
// connection manager if applicable, and it does not already exist.
func patchHCMWithJwtAuthnFilter(mgr *hcm.HttpConnectionManager, irListener *ir.HTTPListener) error {
	if mgr == nil {
		return errors.New("hcm is nil")
	}

	if irListener == nil {
		return errors.New("ir listener is nil")
	}

	if !listenerContainsJwtAuthn(irListener) {
		return nil
	}

	// Return early if filter already exists.
	for _, httpFilter := range mgr.HttpFilters {
		if httpFilter.Name == jwtAuthenFilter {
			return nil
		}
	}

	jwtFilter, err := buildJwtAuthnFilter(irListener)
	if err != nil {
		return err
	}

	// Make sure the router filter is the terminal filter in the chain
	mgr.HttpFilters = append([]*hcm.HttpFilter{jwtFilter}, mgr.HttpFilters...)

	return nil
}

func buildJwtAuthnFilter(irListener *ir.HTTPListener) (*hcm.HttpFilter, error) {
	jwtAuthnProto, err := buildJwtAuthn(irListener)
	if err != nil {
		return nil, err
	}

	if err := jwtAuthnProto.ValidateAll(); err != nil {
		return nil, err
	}

	jwtAuthnFilterAny, err := anypb.New(jwtAuthnProto)
	if err != nil {
		return nil, err
	}

	return &hcm.HttpFilter{
		Name: jwtAuthenFilter,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: jwtAuthnFilterAny,
		},
	}, nil
}

// buildJwtAuthn returns a JwtAuthentication based on the provided IR HTTPRoute.
func buildJwtAuthn(irListener *ir.HTTPListener) (*jwtext.JwtAuthentication, error) {
	jwtProviders := make(map[string]*jwtext.JwtProvider)
	var reqs []*jwtext.JwtRequirement

	irProviders := uniqueJwtAuthnProviders(irListener)

	if len(irProviders) == 0 {
		return nil, fmt.Errorf("listener %s contains no jwt authn providers", irListener.Name)
	}

	for _, irProvider := range irProviders {
		cluster, err := newJwksCluster(&irProvider)
		if err != nil {
			return nil, err
		}

		remote := &jwtext.JwtProvider_RemoteJwks{
			RemoteJwks: &jwtext.RemoteJwks{
				HttpUri: &core.HttpUri{
					Uri: irProvider.RemoteJWKS.URI,
					HttpUpstreamType: &core.HttpUri_Cluster{
						Cluster: cluster.name,
					},
					Timeout: &durationpb.Duration{Seconds: 5},
				},
				CacheDuration: &durationpb.Duration{Seconds: 5 * 60},
			},
		}

		provider := &jwtext.JwtProvider{
			Issuer:              irProvider.Issuer,
			Audiences:           irProvider.Audiences,
			JwksSourceSpecifier: remote,
			PayloadInMetadata:   irProvider.Issuer,
		}

		jwtProviders[irProvider.Name] = provider

		reqs = append(reqs, &jwtext.JwtRequirement{
			RequiresType: &jwtext.JwtRequirement_ProviderName{
				ProviderName: irProvider.Name,
			},
		})
	}

	if len(irProviders) == 1 {
		return &jwtext.JwtAuthentication{
			Rules: []*jwtext.RequirementRule{
				{
					Match: &routev3.RouteMatch{
						PathSpecifier: &routev3.RouteMatch_Prefix{
							Prefix: "/",
						},
					},
					RequirementType: &jwtext.RequirementRule_Requires{
						Requires: reqs[0],
					},
				},
			},
			Providers: jwtProviders,
		}, nil
	}

	return &jwtext.JwtAuthentication{
		Rules: []*jwtext.RequirementRule{
			{
				Match: &routev3.RouteMatch{
					PathSpecifier: &routev3.RouteMatch_Prefix{
						Prefix: "/",
					},
				},
				RequirementType: &jwtext.RequirementRule_Requires{
					Requires: &jwtext.JwtRequirement{
						RequiresType: &jwtext.JwtRequirement_RequiresAny{
							RequiresAny: &jwtext.JwtRequirementOrList{
								Requirements: reqs,
							},
						},
					},
				},
			},
		},
		Providers: jwtProviders,
	}, nil
}

// buildClusterFromJwks creates a xDS Cluster from the provided jwks.
func buildClusterFromJwks(jwks *jwksCluster) (*cluster.Cluster, error) {
	if jwks == nil {
		return nil, errors.New("jwks is nil")
	}

	endpoints, err := jwks.toLbEndpoints()
	if err != nil {
		return nil, err
	}

	tSocket, err := buildXdsUpstreamTLSSocket()
	if err != nil {
		return nil, err
	}

	return &cluster.Cluster{
		Name:                 jwks.name,
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STATIC},
		ConnectTimeout:       durationpb.New(10 * time.Second),
		LbPolicy:             cluster.Cluster_RANDOM,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: jwks.name,
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{
					LbEndpoints: endpoints,
				},
			},
		},
		Http2ProtocolOptions: &core.Http2ProtocolOptions{},
		DnsRefreshRate:       durationpb.New(30 * time.Second),
		RespectDnsTtl:        true,
		DnsLookupFamily:      cluster.Cluster_V4_ONLY,
		TransportSocket:      tSocket,
	}, nil
}

func buildXdsUpstreamTLSSocket() (*core.TransportSocket, error) {
	tlsCtx := &tls.UpstreamTlsContext{
		CommonTlsContext: &tls.CommonTlsContext{
			ValidationContextType: &tls.CommonTlsContext_ValidationContext{
				ValidationContext: &tls.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_Filename{
							Filename: "/etc/ssl/certs/ca-certificates.crt",
						},
					},
				},
			},
		},
	}

	tlsCtxAny, err := anypb.New(tlsCtx)
	if err != nil {
		return nil, err
	}

	return &core.TransportSocket{
		Name: wellknown.TransportSocketTls,
		ConfigType: &core.TransportSocket_TypedConfig{
			TypedConfig: tlsCtxAny,
		},
	}, nil
}

type jwksCluster struct {
	name      string
	addresses []string
	port      int
}

// newJwksCluster returns a jwksCluster from the provided provider.
func newJwksCluster(provider *v1alpha1.JwtAuthenticationFilterProvider) (*jwksCluster, error) {
	if provider == nil {
		return nil, errors.New("nil provider")
	}

	u, err := url.Parse(provider.RemoteJWKS.URI)
	if err != nil {
		return nil, err
	}

	var strPort string
	switch u.Scheme {
	case "http":
		strPort = "80"
	case "https":
		strPort = "443"
	default:
		return nil, fmt.Errorf("unsupported JWKS URI scheme %s", u.Scheme)
	}

	if u.Port() != "" {
		strPort = u.Port()
	}

	addrs, err := resolveHostname(u.Host)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%s_%s", strings.ReplaceAll(u.Host, ".", "_"), strPort)

	port, err := strconv.Atoi(strPort)
	if err != nil {
		return nil, err
	}

	return &jwksCluster{
		name:      name,
		addresses: addrs,
		port:      port,
	}, nil
}

func (j *jwksCluster) toLbEndpoints() ([]*endpoint.LbEndpoint, error) {
	var endpoints []*endpoint.LbEndpoint

	if j == nil {
		return endpoints, errors.New("nil jwks cluster")
	}

	for _, addr := range j.addresses {
		ep := &endpoint.LbEndpoint{
			HostIdentifier: &endpoint.LbEndpoint_Endpoint{
				Endpoint: &endpoint.Endpoint{
					Address: &core.Address{
						Address: &core.Address_SocketAddress{
							SocketAddress: &core.SocketAddress{
								Address:       addr,
								PortSpecifier: &core.SocketAddress_PortValue{PortValue: uint32(j.port)},
							},
						},
					},
				},
			},
		}
		endpoints = append(endpoints, ep)
	}

	return endpoints, nil
}

// resolveHostname looks up the provided hostname using the local resolver, returning the
// resolved IP addresses. If the hostname can't be resolved, hostname will be parsed as an
// IP address
func resolveHostname(hostname string) ([]string, error) {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// Check if hostname is an IPv4 address.
		if ip := net.ParseIP(hostname); ip != nil {
			if v4 := ip.To4(); v4 != nil {
				return []string{v4.String()}, nil
			}
		}
		// Not an IP address or a hostname that can be resolved.
		return nil, fmt.Errorf("failed to parse hostname: %v", err)
	}

	// Only return the hostname's IPv4 addresses.
	var ret []string
	for i := range ips {
		ip := ips[i]
		if v4 := ip.To4(); v4 != nil {
			ret = append(ret, ip.String())
		}
	}

	if ret == nil {
		return nil, fmt.Errorf("hostname %s does not resolve to an IPv4 address", hostname)
	}

	return ret, nil
}

// listenerContainsJwtAuthn returns true if JWT authentication exists for the
// provided listener.
func listenerContainsJwtAuthn(irListener *ir.HTTPListener) bool {
	if irListener == nil {
		return false
	}

	for _, route := range irListener.Routes {
		if routeContainsJwtAuthn(route) {
			return true
		}
	}

	return false
}

// routeContainsJwtAuthn returns true if JWT authentication exists for the
// provided route.
func routeContainsJwtAuthn(irRoute *ir.HTTPRoute) bool {
	if irRoute == nil {
		return false
	}

	if irRoute != nil &&
		irRoute.RequestAuthentication != nil &&
		irRoute.RequestAuthentication.JWT != nil &&
		len(irRoute.RequestAuthentication.JWT.Providers) > 0 {
		return true
	}

	return false
}

func uniqueJwtAuthnProviders(listener *ir.HTTPListener) []v1alpha1.JwtAuthenticationFilterProvider {
	var providers []v1alpha1.JwtAuthenticationFilterProvider

	if listener == nil ||
		len(listener.Routes) == 0 {
		return providers
	}

	// Ignore the provider name when comparing providers.
	opts := cmpopts.IgnoreFields(v1alpha1.JwtAuthenticationFilterProvider{}, "Name")

	for _, route := range listener.Routes {
		if route == nil ||
			route.RequestAuthentication == nil ||
			route.RequestAuthentication.JWT == nil {
			return providers
		}
		for _, provider := range route.RequestAuthentication.JWT.Providers {
			// Skip this provider if it's been created from another route.
			if providers == nil {
				providers = append(providers, provider)
			} else {
				found := false
				for _, p := range providers {
					if cmp.Equal(p, provider, opts) {
						found = true
						break
					}
				}
				if !found {
					providers = append(providers, provider)
				}
			}
		}
	}
	return providers
}
