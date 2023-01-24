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
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/envoyproxy/gateway/internal/ir"
)

const jwtAuthenFilter = "envoy.filters.http.jwt_authn"

// patchRouteWithFilterConfig patches the provided xDS route with a TypedPerFilterConfig, if needed.
// The following TypedPerFilterConfigs are supported:
//   - jwtAuthenFilter
func patchRouteWithFilterConfig(route *routev3.Route, irRoute *ir.HTTPRoute) error { //nolint:unparam
	if route == nil {
		return errors.New("xds route is nil")
	}
	if irRoute == nil {
		return errors.New("ir route is nil")
	}

	cfg := route.GetTypedPerFilterConfig()
	if _, ok := cfg[jwtAuthenFilter]; !ok {
		if !isJwtAuthnPresent(irRoute) {
			return nil
		}

		jwtAuthn, err := buildJwtAuthn(irRoute)
		if err != nil {
			return err
		}

		jwtFilterProto, err := anypb.New(jwtAuthn)
		if err != nil {
			return err
		}

		if cfg == nil {
			route.TypedPerFilterConfig = make(map[string]*anypb.Any)
		}

		route.TypedPerFilterConfig[jwtAuthenFilter] = jwtFilterProto
	}

	return nil
}

// isJwtAuthnPresent returns true if JWT authentication exists for the provided IR HTTPRoute.
func isJwtAuthnPresent(irRoute *ir.HTTPRoute) bool {
	if irRoute != nil &&
		irRoute.RequestAuthentication != nil &&
		irRoute.RequestAuthentication.JWT != nil &&
		len(irRoute.RequestAuthentication.JWT.Providers) > 0 {
		return true
	}

	return false
}

// buildJwtAuthn returns a JwtAuthentication based on the provided IR HTTPRoute.
func buildJwtAuthn(irRoute *ir.HTTPRoute) (*jwtext.JwtAuthentication, error) {
	providers := map[string]*jwtext.JwtProvider{}
	reqs := make(map[string]*jwtext.JwtRequirement)

	for i := range irRoute.RequestAuthentication.JWT.Providers {
		irProvider := irRoute.RequestAuthentication.JWT.Providers[i]
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

		providers[irProvider.Name] = provider

		reqs[irProvider.Name] = &jwtext.JwtRequirement{
			RequiresType: &jwtext.JwtRequirement_ProviderName{
				ProviderName: irProvider.Name,
			},
		}
	}

	return &jwtext.JwtAuthentication{
		RequirementMap: reqs,
		Providers:      providers,
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

	return &cluster.Cluster{
		Name:                 jwks.name,
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STRICT_DNS},
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
