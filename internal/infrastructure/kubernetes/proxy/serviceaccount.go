package proxy

import (
	"context"
	"errors"
	"fmt"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/envoyproxy/gateway/internal/ir"
)

// CreateServiceAccount creates the proxy serviceAccount if it doesn't exist. If
// the serviceAccount exists but is not the expected configuration, it will be
// updated accordingly.
func (c *Infra) CreateServiceAccount(ctx context.Context, infra *ir.Infra) error {
	if infra == nil {
		return errors.New("infra ir is nil")
	}

	if c.Resources[KindServiceAccount] == nil {
		sa := c.expectedServiceAccount(infra)
		err := c.createServiceAccount(ctx, sa)
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				c.updateServiceAccountResource(sa)
				return nil
			}
			return err
		}
		c.addResource(KindServiceAccount, sa)
		return nil
	}

	// The ServiceAccount kind exists, make sure it matches the expected state.
	current, err := c.currentServiceAccount(ctx, infra.Proxy.Namespace, infra.Proxy.Name)
	if err != nil {
		if kerrors.IsNotFound(err) {
			sa := c.expectedServiceAccount(infra)
			c.createServiceAccount(ctx, sa)
		}
		return fmt.Errorf("failed to get serviceacount %s/%s", infra.Proxy.Namespace, infra.Proxy.Name)
	}
	for i := range c.Resources[KindServiceAccount] {
		obj := c.Resources[KindServiceAccount][i]
		if current, ok := obj.(*rbacv1.Role); !ok {
			return fmt.Errorf()
		}
	}

	return c.updateRoleIfNeeded(ctx, current, c.expectedServiceAccount(infra))
}

// expectedServiceAccount returns the expected proxy serviceAccount based on the provided infra.
func (c *Infra) expectedServiceAccount(infra *ir.Infra) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: infra.Proxy.Namespace,
			Name:      infra.Proxy.Name,
		},
	}
}

// getServiceAccount returns the ServiceAccount resource for the provided infra.
func (c *Infra) getServiceAccount(ctx context.Context, infra *ir.Infra) (*corev1.ServiceAccount, error) {
	ns := infra.Proxy.Namespace
	name := infra.Proxy.Name
	key := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	sa := new(corev1.ServiceAccount)
	if err := c.client.Get(ctx, key, sa); err != nil {
		return nil, fmt.Errorf("failed to get serviceAccount %s/%s: %w", ns, name, err)
	}

	return sa, nil
}

// createServiceAccount creates a ServiceAccount resource for the provided sa.
func (c *Infra) createServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) error {
	if err := c.client.Create(ctx, sa); err != nil {
		return fmt.Errorf("failed to create serviceAccount %s/%s: %w", sa.Namespace, sa.Name, err)
	}
	return nil
}
