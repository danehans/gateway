package infrastructure

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/envoyproxy/gateway/api/config/v1alpha1"
	"github.com/envoyproxy/gateway/internal/envoygateway/config"
	"github.com/envoyproxy/gateway/internal/infrastructure/kubernetes"
)

func StartManager(svr *config.Server) error {
	log := svr.Logger
	if svr.EnvoyGateway.Provider.Type == v1alpha1.ProviderTypeKubernetes {
		log.Info("Using provider", "type", v1alpha1.ProviderTypeKubernetes)
		cfg := ctrl.GetConfigOrDie()
		mgr, err := kubernetes.NewManager(cfg, svr)
		if err != nil {
			return fmt.Errorf("failed to create %s infra manager", v1alpha1.ProviderTypeKubernetes)
		}
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			return fmt.Errorf("failed to start %s infra manager", v1alpha1.ProviderTypeKubernetes)
		}
	}
	// Unsupported provider.
	return fmt.Errorf("unsupported infra manager type %v", svr.EnvoyGateway.Provider.Type)
}
