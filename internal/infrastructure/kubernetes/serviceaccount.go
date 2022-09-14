package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/envoyproxy/gateway/internal/ir"
)

const (
	envoyServiceAccountName = "envoy"
)

// expectedServiceAccount returns the expected proxy serviceAccount.
func (im *Infra) expectedServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: im.Namespace,
			Name:      envoyServiceAccountName,
		},
	}
}

// createOrUpdateServiceAccount creates the Envoy ServiceAccount in the kube api server,
// if it doesn't exist and updates it if it does.
func (im *Infra) createOrUpdateServiceAccount(ctx context.Context, _ *ir.Infra) error {
	sa := im.expectedServiceAccount()

	current := &corev1.ServiceAccount{}
	key := types.NamespacedName{
		Namespace: im.Namespace,
		Name:      envoyServiceAccountName,
	}

	if err := im.Client.Get(ctx, key, current); err != nil {
		if kerrors.IsNotFound(err) {
			// Create if it does not exist.
			if err := im.Client.Create(ctx, sa); err != nil {
				return fmt.Errorf("failed to create serviceaccount %s/%s: %w",
					sa.Namespace, sa.Name, err)
			}
		}
	} else {
		// Since the ServiceAccount does not have a specific Spec field to compare
		// just perform an update for now.
		if err := im.Client.Update(ctx, sa); err != nil {
			return fmt.Errorf("failed to update serviceaccount %s/%s: %w",
				sa.Namespace, sa.Name, err)
		}
	}

	if err := im.updateResource(sa); err != nil {
		return err
	}

	return nil
}

// deleteServiceAccount deletes the Envoy ServiceAccount in the kube api server,
// if it exists.
func (im *Infra) deleteServiceAccount(ctx context.Context) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: im.Namespace,
			Name:      envoyServiceAccountName,
		},
	}
	if err := im.Client.Delete(ctx, sa); err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete serviceaccount %s/%s: %w", sa.Namespace, sa.Name, err)
	}

	return nil
}
