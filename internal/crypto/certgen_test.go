package crypto

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/envoyproxy/gateway/api/config/v1alpha1"
)

func TestGenerateCerts(t *testing.T) {
	type testcase struct {
		envoyGateway            *v1alpha1.EnvoyGateway
		certConfig              *Configuration
		wantEnvoyGatewayDNSName string
		wantEnvoyDNSName        string
		wantError               error
	}

	run := func(t *testing.T, name string, tc testcase) {
		t.Helper()

		t.Run(name, func(t *testing.T) {
			t.Helper()

			got, err := GenerateCerts(tc.certConfig, tc.envoyGateway)

			// Note we don't match error string values
			// because the actual values come from Kubernetes
			// internals and may not be stable.
			switch {
			case tc.wantError == nil && err != nil:
				t.Errorf("wanted no error, got error %q", err)
			case tc.wantError != nil && err == nil:
				t.Error("wanted error, got no error")
			case tc.wantError == nil && err == nil:
				// If using a custom lifetime, validate the certs
				// as of an hour before the intended expiration.
				currentTime := time.Now()
				if tc.certConfig.Lifetime != 0 {
					currentTime = currentTime.Add(24 * time.Hour * time.Duration(tc.certConfig.Lifetime)).Add(-time.Hour)
				}

				roots := x509.NewCertPool()
				ok := roots.AppendCertsFromPEM(got.CACertificate)
				require.Truef(t, ok, "Failed to set up CA cert for testing, maybe it's an invalid PEM")

				err = verifyCert(got.EnvoyGatewayCertificate, roots, tc.wantEnvoyGatewayDNSName, currentTime)
				assert.NoErrorf(t, err, "Validating %s failed", name)

				err = verifyCert(got.EnvoyCertificate, roots, tc.wantEnvoyDNSName, currentTime)
				assert.NoErrorf(t, err, "Validating %s failed", name)
			}
		})
	}

	run(t, "no configuration - use defaults", testcase{
		certConfig:              &Configuration{},
		wantEnvoyGatewayDNSName: "envoy-gateway",
		wantEnvoyDNSName:        "envoy",
		wantError:               nil,
	})

	run(t, "custom service names", testcase{
		certConfig: &Configuration{
			EnvoyGatewayDNSPrefix: "custom-eg",
			EnvoyDNSPrefix:        "custom-envoy",
		},
		wantEnvoyGatewayDNSName: "custom-eg",
		wantEnvoyDNSName:        "custom-envoy",
		wantError:               nil,
	})

	run(t, "custom namespace", testcase{
		certConfig: &Configuration{
			Namespace: "custom-namespace",
		},
		wantEnvoyGatewayDNSName: "envoy-gateway",
		wantEnvoyDNSName:        "envoy",
		wantError:               nil,
	})

	run(t, "custom lifetime", testcase{
		certConfig: &Configuration{
			// use a lifetime longer than the default so we
			// can verify that it's taking effect by validating
			// the certs as of a time after the default expiration.
			Lifetime: DefaultCertificateLifetime * 2,
		},
		wantEnvoyGatewayDNSName: "envoy-gateway",
		wantEnvoyDNSName:        "envoy",
		wantError:               nil,
	})

	run(t, "custom dns name", testcase{
		certConfig: &Configuration{
			DNSName: "custom.dns.name",
		},
		wantEnvoyGatewayDNSName: "envoy-gateway",
		wantEnvoyDNSName:        "envoy",
		wantError:               nil,
	})

	run(t, "unsupported envoy gateway provider", testcase{
		envoyGateway: &v1alpha1.EnvoyGateway{
			EnvoyGatewaySpec: v1alpha1.EnvoyGatewaySpec{
				Provider: &v1alpha1.Provider{
					Type: v1alpha1.ProviderTypeFile,
				},
			},
		},
		wantError: errors.New("unsupported provider type File"),
	})
}

func TestGeneratedValidKubeCerts(t *testing.T) {

	now := time.Now()
	expiry := now.Add(24 * 365 * time.Hour)

	caCert, caKey, err := newCA("envoy-gateway", expiry)
	require.NoErrorf(t, err, "Failed to generate CA cert")

	egCertReq := &certificateRequest{
		caCertPEM:  caCert,
		caKeyPEM:   caKey,
		expiry:     expiry,
		commonName: "envoy-gateway",
		altNames:   kubeServiceNames("envoy-gateway", "envoy-gateway-system", "cluster.local"),
	}
	egCert, _, err := newCert(egCertReq)
	require.NoErrorf(t, err, "Failed to generate Envoy Gateway cert")

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(caCert)
	require.Truef(t, ok, "Failed to set up CA cert for testing, maybe it's an invalid PEM")

	envoyCertReq := &certificateRequest{
		caCertPEM:  caCert,
		caKeyPEM:   caKey,
		expiry:     expiry,
		commonName: "envoy",
		altNames:   kubeServiceNames("envoy", "envoy-gateway-system", "cluster.local"),
	}
	envoyCert, _, err := newCert(envoyCertReq)
	require.NoErrorf(t, err, "Failed to generate Envoy cert")

	tests := map[string]struct {
		cert    []byte
		dnsName string
	}{
		"envoy gateway cert": {
			cert:    egCert,
			dnsName: "envoy-gateway",
		},
		"envoy cert": {
			cert:    envoyCert,
			dnsName: "envoy",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := verifyCert(tc.cert, roots, tc.dnsName, now)
			assert.NoErrorf(t, err, "Validating %s failed", name)
		})
	}

}

func verifyCert(certPEM []byte, roots *x509.CertPool, dnsname string, currentTime time.Time) error {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode %s certificate from PEM form", dnsname)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	opts := x509.VerifyOptions{
		DNSName:     dnsname,
		Roots:       roots,
		CurrentTime: currentTime,
	}
	if _, err = cert.Verify(opts); err != nil {
		return fmt.Errorf("certificate verification failed: %s", err)
	}

	return nil
}
