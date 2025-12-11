package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

var (
	testenv              env.Environment
	kindClusterName      string
	namespace            = "podlock"
	nriImage             = "ghcr.io/flavio/podlock/nri:latest"
	controllerImage      = "ghcr.io/flavio/podlock/controller:latest"
	certManagerNamespace = "cert-manager"
	certManagerVersion   = "v1.18.2"
)

func TestMain(m *testing.M) {
	cfg, _ := envconf.NewFromFlags()
	testenv = env.NewWithConfig(cfg)
	kindClusterName = envconf.RandomName("podlock-e2e-cluster", 32)

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),
		envfuncs.CreateNamespace(namespace),
		envfuncs.LoadImageToCluster(kindClusterName, nriImage, "--verbose", "--mode", "direct"),
		envfuncs.LoadImageToCluster(kindClusterName, controllerImage, "--verbose", "--mode", "direct"),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			manager := helm.New(cfg.KubeconfigFile())

			// Add the Jetstack Helm repository for cert-manager
			err := manager.RunRepo(helm.WithArgs(
				"add",
				"jetstack",
				"https://charts.jetstack.io",
				"--force-update"),
			)
			if err != nil {
				return ctx, fmt.Errorf("failed to add cert-manager helm repo: %w", err)
			}

			// Install cert-manager
			err = manager.RunInstall(
				helm.WithName("cert-manager"),
				helm.WithChart("jetstack/cert-manager"),
				helm.WithWait(),
				helm.WithArgs("--version", certManagerVersion),
				helm.WithArgs("--set", "installCRDs=true"),
				helm.WithNamespace(certManagerNamespace),
				helm.WithArgs("--create-namespace"),
				helm.WithTimeout("3m"))
			if err != nil {
				return ctx, fmt.Errorf("failed to install cert-manager: %w", err)
			}

			return ctx, nil
		},
	)

	testenv.Finish(
		envfuncs.ExportClusterLogs(kindClusterName, "./logs"),
		envfuncs.DestroyCluster(kindClusterName),
	)

	os.Exit(testenv.Run(m))
}
