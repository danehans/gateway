package kubernetes

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/envoyproxy/gateway/internal/envoygateway/config"
	"github.com/envoyproxy/gateway/internal/gatewayapi"
	"github.com/envoyproxy/gateway/internal/ir"
)

// expectedServices returns the expected Services based on the provided infra.
func (im *Infra) expectedServices(infra *ir.Infra) ([]*corev1.Service, error) {
	var svcs []*corev1.Service
	for _, listener := range infra.Proxy.Listeners {
		var ports []corev1.ServicePort
		for _, port := range listener.Ports {
			target := intstr.IntOrString{IntVal: port.ContainerPort}
			p := corev1.ServicePort{
				Name:       port.Name,
				Protocol:   corev1.ProtocolTCP,
				Port:       port.ServicePort,
				TargetPort: target,
			}
			ports = append(ports, p)
		}
		// Set the labels based on the owning gatewayclass name.
		labels := envoyLabels(infra.GetProxyInfra().GetProxyMetadata().Labels)
		if _, ok := labels[gatewayapi.OwningGatewayClassLabel]; !ok {
			return nil, fmt.Errorf("missing owning gatewayclass label")
		}
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: im.Namespace,
				Name:      fmt.Sprintf("%s-%s", config.EnvoyServiceName, listener.Name),
				Labels:    labels,
			},
			Spec: corev1.ServiceSpec{
				Type:            corev1.ServiceTypeLoadBalancer,
				Ports:           ports,
				Selector:        envoySelector(infra.GetProxyInfra().GetProxyMetadata().Labels).MatchLabels,
				SessionAffinity: corev1.ServiceAffinityNone,
				// Preserve the client source IP and avoid a second hop for LoadBalancer.
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			},
		}
		svcs = append(svcs, svc)
	}

	return svcs, nil
}

// createOrUpdateServices creates one or more Services in the kube api server
// based on the provided infra, if they don't exist or updates they do.
func (im *Infra) createOrUpdateServices(ctx context.Context, infra *ir.Infra) error {
	svcs, err := im.expectedServices(infra)
	if err != nil {
		return fmt.Errorf("failed to generate expected services: %w", err)
	}

	for _, svc := range svcs {
		current := new(corev1.Service)
		key := types.NamespacedName{
			Namespace: svc.Namespace,
			Name:      fmt.Sprintf("%s-%s", config.EnvoyServiceName, svc.Name),
		}

		if err := im.Client.Get(ctx, key, current); err != nil {
			// Create if not found.
			if kerrors.IsNotFound(err) {
				if err := im.Client.Create(ctx, svc); err != nil {
					// TODO: Understand why a "Create" occurs when using multiple Gateways.
					if kerrors.IsAlreadyExists(err) {
						return nil
					}
					return fmt.Errorf("failed to create service %s/%s: %w",
						svc.Namespace, svc.Name, err)
				}
			}
		} else {
			// Update if current value is different.
			if !reflect.DeepEqual(svc.Spec, current.Spec) {
				if err := im.Client.Update(ctx, svc); err != nil {
					return fmt.Errorf("failed to update service %s/%s: %w",
						svc.Namespace, svc.Name, err)
				}
			}
		}

		if err := im.updateResource(svc); err != nil {
			return err
		}
	}

	return nil
}

// deleteServices deletes the Envoy Services in the kube api server, if it exists.
func (im *Infra) deleteServices(ctx context.Context) error {
	svcList := corev1.ServiceList{}
	if err := im.Client.List(ctx, &svcList, client.InNamespace(im.Namespace)); err != nil {
		return fmt.Errorf("failed listing services: %w", err)
	}

	for i := range svcList.Items {
		svc := svcList.Items[i]
		if err := im.Client.Delete(ctx, &svc); err != nil {
			if kerrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to delete service %s/%s: %w", svc.Namespace, svc.Name, err)
		}
	}

	return nil
}
