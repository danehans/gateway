package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/envoyproxy/gateway/internal/crypto"
	"github.com/envoyproxy/gateway/internal/ir"
)

const (
	// caCertificateKey is the key name for accessing TLS CA certificate bundles
	// in Kubernetes Secrets.
	caCertificateKey = "ca.crt"
)

// createSecretIfNeeded creates a Secret based on the provided infra, if
// it doesn't exist in the kube api server.
func (i *Infra) createSecretIfNeeded(ctx context.Context, infra *ir.Infra) error {
	current, err := i.getSecret(ctx, infra)
	if err != nil {
		if kerrors.IsNotFound(err) {
			secret, err := i.createSecret(ctx, infra)
			if err != nil {
				return err
			}
			if err := i.addResource(secret); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	if err := i.addResource(current); err != nil {
		return err
	}

	return nil
}

// getSecret gets the Secret for the provided infra from the kube api.
func (i *Infra) getSecret(ctx context.Context, infra *ir.Infra) (*corev1.Secret, error) {
	ns := i.Namespace
	name := infra.Proxy.Name
	key := types.NamespacedName{
		Namespace: ns,
		Name:      infra.GetProxyInfra().ProxyName(),
	}
	secret := new(corev1.Secret)
	if err := i.Client.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", ns, name, err)
	}

	return secret, nil
}

// expectedSecret returns the expected proxy serviceAccount based on the provided infra.
func (i *Infra) expectedSecret(infra *ir.Infra) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: i.Namespace,
			Name:      infra.GetProxyInfra().Name,
		},
		Data:       nil,
		StringData: nil,
		Type:       corev1.SecretTypeTLS,
	}
}

// AsSecrets transforms certData into a slice of Secrets in compact Secret format,
// which is compatible with both cert-manager and Envoy Gateway.
func AsSecrets(namespace, nameSuffix string, certData *crypto.Certificates) ([]*corev1.Secret, []error) {
	if errs := validateSecretNamespaceAndName(namespace, "envoycert"+nameSuffix); len(errs) > 0 {
		return nil, errs
	}

	return []*corev1.Secret{
		newSecret(
			corev1.SecretTypeTLS,
			"envoycert"+nameSuffix,
			namespace,
			map[string][]byte{
				caCertificateKey:        certData.CACertificate,
				corev1.TLSCertKey:       certData.EnvoyCertificate,
				corev1.TLSPrivateKeyKey: certData.EnvoyPrivateKey,
			}),
	}, nil
}

// createServiceAccount creates a Secret in the kube api server based on the provided infra,
// if it doesn't exist.
func (i *Infra) createSecret(ctx context.Context, infra *ir.Infra) (*corev1.Secret, error) {
	expected := i.expectedSecret(infra)
	err := i.Client.Create(ctx, expected)
	if err != nil {
		if kerrors.IsAlreadyExists(err) {
			return expected, nil
		}
		return nil, fmt.Errorf("failed to create secret %s/%s: %w",
			expected.Namespace, expected.Name, err)
	}

	return expected, nil
}

func newSecret(secretType corev1.SecretType, name string, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		Type: secretType,
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "Envoy Gateway",
			},
		},
		Data: data,
	}
}

func validateSecretNamespaceAndName(namespace, name string) []error {
	var errs []error

	for _, labelErr := range validation.IsDNS1123Label(namespace) {
		errs = append(errs, fmt.Errorf("invalid namespace name %q: %s", namespace, labelErr))
	}

	for _, domainErr := range validation.IsDNS1123Subdomain(name) {
		errs = append(errs, fmt.Errorf("invalid secret name %q: %s", name, domainErr))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
